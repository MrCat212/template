package positions

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/MrCat212/template/internal/domain/trip"
)

const (
	resultQueued    = "queued"
	resultDuplicate = "duplicate"
	resultDropped   = "dropped"
)

// Config determines how often to scan and how many requests run concurrently.
type Config struct {
	CheckInterval  time.Duration
	StaleAfter     time.Duration
	Workers        int
	QueueSize      int
	RequestTimeout time.Duration
}

type tripFinder interface {
	FindStaleTrips(context.Context, time.Duration) ([]trip.StaleTrip, error)
}

type positionRequester interface {
	RequestPosition(context.Context, trip.StaleTrip) error
}

// Ticker makes the scan clock replaceable in tests.
type Ticker interface {
	Chan() <-chan time.Time
	Stop()
}

type realTicker struct{ ticker *time.Ticker }

func (t realTicker) Chan() <-chan time.Time { return t.ticker.C }
func (t realTicker) Stop()                  { t.ticker.Stop() }

type task struct {
	trip    trip.StaleTrip
	version uint64
	link    trace.SpanContext
}

type pendingTask struct {
	version uint64
	cancel  context.CancelFunc
}

// Pool runs one scanner and a fixed number of workers.
type Pool struct {
	finder    tripFinder
	requester positionRequester
	logger    *slog.Logger
	metrics   *poolMetrics
	cfg       Config
	ticker    Ticker

	queue       chan task
	mu          sync.Mutex
	pending     map[uuid.UUID]*pendingTask
	nextVersion uint64

	startOnce     sync.Once
	stopOnce      sync.Once
	scannerCancel context.CancelFunc
	workerCancel  context.CancelFunc
	scannerDone   chan struct{}
	stopped       chan struct{}
	workers       sync.WaitGroup
	workerCtx     context.Context
	scannerCtx    context.Context
}

func New(finder tripFinder, requester positionRequester, logger *slog.Logger, cfg Config, ticker Ticker) (*Pool, error) {
	if finder == nil || requester == nil {
		return nil, fmt.Errorf("position finder and requester must be non-nil")
	}
	if cfg.CheckInterval <= 0 || cfg.StaleAfter <= 0 || cfg.RequestTimeout <= 0 || cfg.Workers <= 0 || cfg.QueueSize <= 0 {
		return nil, fmt.Errorf("invalid position worker configuration")
	}
	if logger == nil {
		logger = slog.Default()
	}
	metrics, err := newPoolMetrics()
	if err != nil {
		return nil, err
	}
	return &Pool{
		finder: finder, requester: requester, logger: logger, metrics: metrics, cfg: cfg, ticker: ticker,
		queue:       make(chan task, cfg.QueueSize),
		pending:     make(map[uuid.UUID]*pendingTask),
		scannerDone: make(chan struct{}), stopped: make(chan struct{}),
	}, nil
}

// Start launches a single scanner and exactly cfg.Workers worker goroutines.
func (p *Pool) Start() {
	p.startOnce.Do(func() {
		p.scannerCtx, p.scannerCancel = context.WithCancel(context.Background())
		p.workerCtx, p.workerCancel = context.WithCancel(context.Background())
		if p.ticker == nil {
			p.ticker = realTicker{ticker: time.NewTicker(p.cfg.CheckInterval)}
		}
		p.workers.Add(p.cfg.Workers)
		for range p.cfg.Workers {
			go p.worker()
		}
		go p.scanner()
	})
}

func (p *Pool) scanner() {
	defer close(p.scannerDone)
	defer p.ticker.Stop()

	for {
		select {
		case <-p.scannerCtx.Done():
			return
		case _, ok := <-p.ticker.Chan():
			if !ok || p.scannerCtx.Err() != nil {
				return
			}
			p.scanOnce(p.scannerCtx)
		}
	}
}

// scanOnce is synchronous: two scans never overlap on the same ticker loop.
func (p *Pool) scanOnce(ctx context.Context) {
	ctx, span := otel.Tracer("trip-service.worker.positions").Start(ctx, "positions.scan")
	defer span.End()

	items, err := p.finder.FindStaleTrips(ctx, p.cfg.StaleAfter)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		p.logger.ErrorContext(ctx, "scan stale trips", "error", err)
		return
	}

	for _, item := range items {
		result := p.enqueue(ctx, item)
		p.metrics.recordEnqueue(ctx, result)
		if result == resultDropped {
			p.logger.WarnContext(ctx, "position queue full; dropping task", "trip_id", item.ID.String())
		}
	}
}

// enqueue never blocks, even when all workers are busy.
func (p *Pool) enqueue(ctx context.Context, item trip.StaleTrip) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.pending[item.ID]; exists {
		return resultDuplicate
	}

	p.nextVersion++
	job := task{trip: item, version: p.nextVersion, link: trace.SpanContextFromContext(ctx)}
	select {
	case p.queue <- job:
		p.pending[item.ID] = &pendingTask{version: job.version}
		p.metrics.queueSize.Add(ctx, 1)
		return resultQueued
	default:
		return resultDropped
	}
}

// Cancel invalidates queued requests. Buffered tasks are skipped by workers.
// If a request has already started, its context is canceled too.
func (p *Pool) Cancel(tripID uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, exists := p.pending[tripID]
	if !exists {
		return
	}
	delete(p.pending, tripID)
	if entry.cancel != nil {
		entry.cancel()
	} else {
		// No HTTP request has started: account for the skipped queued task.
		p.metrics.recordQueuedCancelled(context.Background())
	}
}

func (p *Pool) worker() {
	defer p.workers.Done()
	for job := range p.queue {
		p.process(job)
	}
}

func (p *Pool) process(job task) {
	p.mu.Lock()
	p.metrics.queueSize.Add(p.workerCtx, -1)
	entry, exists := p.pending[job.trip.ID]
	if !exists || entry.version != job.version {
		p.mu.Unlock()
		return
	}

	options := []trace.SpanStartOption{trace.WithNewRoot()}
	if job.link.IsValid() {
		options = append(options, trace.WithLinks(trace.Link{SpanContext: job.link}))
	}
	ctx, span := otel.Tracer("trip-service.worker.positions").Start(p.workerCtx, "positions.request", options...)
	requestCtx, cancel := context.WithTimeout(ctx, p.cfg.RequestTimeout)
	entry.cancel = cancel
	p.mu.Unlock()

	p.metrics.workersActive.Add(ctx, 1)
	defer p.metrics.workersActive.Add(ctx, -1)
	started := time.Now()
	err := p.requester.RequestPosition(requestCtx, job.trip)
	canceled := requestCtx.Err() != nil
	cancel()

	result := "success"
	if err != nil {
		result = "error"
		if canceled {
			result = "cancelled"
		}
	}
	p.metrics.recordRequest(ctx, result, time.Since(started))

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		p.logger.ErrorContext(ctx, "request position failed", "trip_id", job.trip.ID.String(), "error", err)
	} else {
		p.logger.InfoContext(ctx, "position requested", "trip_id", job.trip.ID.String(), "duration", time.Since(started))
	}
	span.End()

	p.mu.Lock()
	if current, ok := p.pending[job.trip.ID]; ok && current.version == job.version {
		delete(p.pending, job.trip.ID)
	}
	p.mu.Unlock()
}

// Stop first stops the producer, then drains the queue. A deadline cancels
// in-flight HTTP requests so worker goroutines can finish.
func (p *Pool) Stop(ctx context.Context) error {
	p.stopOnce.Do(func() {
		if p.scannerCancel == nil {
			close(p.stopped)
			return
		}
		p.scannerCancel()
		<-p.scannerDone
		close(p.queue)
		go func() {
			p.workers.Wait()
			p.workerCancel()
			close(p.stopped)
		}()
	})

	select {
	case <-p.stopped:
		return ctx.Err()
	case <-ctx.Done():
		if p.workerCancel != nil {
			p.workerCancel()
		}
		<-p.stopped
		return ctx.Err()
	}
}
