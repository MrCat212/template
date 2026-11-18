package positions

import (
	"bytes"
	"container/heap"
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
	resultReplaced  = "replaced" // internal status, never exported as a metric label
)

// Config determines how often to scan and how many requests run concurrently.
type Config struct {
	CheckInterval  time.Duration
	StaleAfter     time.Duration
	Workers        int
	QueueSize      int
	ScanBatchSize  int
	RequestTimeout time.Duration
}

type tripFinder interface {
	FindStaleTripsBatch(context.Context, time.Duration, int, *trip.StaleCursor) ([]trip.StaleTrip, error)
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

// queuedTask tracks its index so Cancel can remove a buffered job in O(log n).
type queuedTask struct {
	job   task
	index int
}

// priorityQueue is a min-heap: the oldest last activity wins.
// For equal timestamps, UUID gives a deterministic order (like the SQL query).
type priorityQueue []*queuedTask

func (q priorityQueue) Len() int { return len(q) }

func older(a, b task) bool {
	if a.trip.LastActivityAt.Equal(b.trip.LastActivityAt) {
		return bytes.Compare(a.trip.ID[:], b.trip.ID[:]) < 0
	}
	return a.trip.LastActivityAt.Before(b.trip.LastActivityAt)
}

func (q priorityQueue) Less(i, j int) bool { return older(q[i].job, q[j].job) }

func (q priorityQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index = i
	q[j].index = j
}

func (q *priorityQueue) Push(value any) {
	item := value.(*queuedTask)
	item.index = len(*q)
	*q = append(*q, item)
}

func (q *priorityQueue) Pop() any {
	old := *q
	last := len(old) - 1
	item := old[last]
	old[last] = nil
	item.index = -1
	*q = old[:last]
	return item
}

type pendingTask struct {
	version uint64
	queued  *queuedTask // nil after a worker takes the job
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

	queue       priorityQueue
	mu          sync.Mutex
	queueCond   *sync.Cond
	queueClosed bool
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
	if cfg.CheckInterval <= 0 || cfg.StaleAfter <= 0 || cfg.RequestTimeout <= 0 || cfg.Workers <= 0 || cfg.QueueSize <= 0 || cfg.ScanBatchSize <= 0 {
		return nil, fmt.Errorf("invalid position worker configuration")
	}
	if logger == nil {
		logger = slog.Default()
	}
	metrics, err := newPoolMetrics()
	if err != nil {
		return nil, err
	}
	p := &Pool{
		finder: finder, requester: requester, logger: logger, metrics: metrics, cfg: cfg, ticker: ticker,
		queue:       make(priorityQueue, 0, cfg.QueueSize),
		pending:     make(map[uuid.UUID]*pendingTask),
		scannerDone: make(chan struct{}), stopped: make(chan struct{}),
	}
	p.queueCond = sync.NewCond(&p.mu)
	heap.Init(&p.queue)
	return p, nil
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

	rowsRead := 0
	defer func() { p.metrics.recordRowsRead(ctx, rowsRead) }()

	var cursor *trip.StaleCursor
	for {
		if ctx.Err() != nil {
			return
		}

		items, err := p.finder.FindStaleTripsBatch(ctx, p.cfg.StaleAfter, p.cfg.ScanBatchSize, cursor)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			p.logger.ErrorContext(ctx, "scan stale trips", "error", err)
			return
		}
		rowsRead += len(items)

		for _, item := range items {
			if ctx.Err() != nil {
				return
			}
			result := p.enqueue(ctx, item)
			if result == resultReplaced {
				// The older trip was accepted, but a newer queued trip was evicted.
				// The first dropped task ends this scan, including eviction.
				p.metrics.recordEnqueue(ctx, resultQueued)
				p.metrics.recordEnqueue(ctx, resultDropped)
				return
			}
			p.metrics.recordEnqueue(ctx, result)
			if result == resultDropped {
				p.logger.WarnContext(ctx, "position queue full; dropping task", "trip_id", item.ID.String())
				return
			}
		}

		if len(items) < p.cfg.ScanBatchSize {
			return
		}

		last := items[len(items)-1]
		cursor = &trip.StaleCursor{LastActivityAt: last.LastActivityAt, ID: last.ID}
	}
}

// enqueue never waits for a worker. On overflow, an older trip displaces
// the *newest* buffered trip; the in-progress requests cannot be evicted.
func (p *Pool) enqueue(ctx context.Context, item trip.StaleTrip) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.queueClosed {
		return resultDropped
	}
	if _, exists := p.pending[item.ID]; exists {
		return resultDuplicate
	}

	p.nextVersion++
	job := task{trip: item, version: p.nextVersion, link: trace.SpanContextFromContext(ctx)}
	entry := &queuedTask{job: job}

	if p.queue.Len() < p.cfg.QueueSize {
		heap.Push(&p.queue, entry)
		p.pending[item.ID] = &pendingTask{version: job.version, queued: entry}
		p.metrics.queueSize.Add(ctx, 1)
		p.queueCond.Signal()
		return resultQueued
	}

	// Heap root is oldest; find its opposite (the newest) in O(queue size).
	// The bounded queue makes this scan finite, with no task goroutines.
	newest := 0
	for i := 1; i < p.queue.Len(); i++ {
		if older(p.queue[newest].job, p.queue[i].job) {
			newest = i
		}
	}
	if !older(job, p.queue[newest].job) {
		return resultDropped
	}

	evicted := heap.Remove(&p.queue, newest).(*queuedTask)
	delete(p.pending, evicted.job.trip.ID)
	heap.Push(&p.queue, entry)
	p.pending[item.ID] = &pendingTask{version: job.version, queued: entry}
	// Queue length did not change: do not modify the size gauge.
	p.logger.WarnContext(ctx, "position queue full; evicted newer task",
		"trip_id", evicted.job.trip.ID.String(), "replacement_trip_id", item.ID.String())
	p.queueCond.Signal()
	return resultReplaced
}

// Cancel physically removes queued jobs; running jobs get context cancellation.
func (p *Pool) Cancel(tripID uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, exists := p.pending[tripID]
	if !exists {
		return
	}
	delete(p.pending, tripID)
	if entry.queued != nil {
		heap.Remove(&p.queue, entry.queued.index)
		p.metrics.queueSize.Add(context.Background(), -1)
		p.metrics.recordQueuedCancelled(context.Background())
		return
	}
	if entry.cancel != nil {
		entry.cancel()
	} else {
		// The worker dequeued this job but has not yet started HTTP.
		p.metrics.recordQueuedCancelled(context.Background())
	}
}

// take sleeps on a condition variable while the queue is empty.
// The scanner/HTTP handlers only signal under the same mutex.
func (p *Pool) take() (task, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for p.queue.Len() == 0 && !p.queueClosed && p.workerCtx.Err() == nil {
		p.queueCond.Wait()
	}
	if p.workerCtx.Err() != nil || p.queue.Len() == 0 {
		return task{}, false
	}
	entry := heap.Pop(&p.queue).(*queuedTask)
	if pending, ok := p.pending[entry.job.trip.ID]; ok && pending.version == entry.job.version {
		pending.queued = nil
	}
	p.metrics.queueSize.Add(p.workerCtx, -1)
	return entry.job, true
}

func (p *Pool) worker() {
	defer p.workers.Done()
	for {
		job, ok := p.take()
		if !ok {
			return
		}
		p.process(job)
	}
}

func (p *Pool) process(job task) {
	p.mu.Lock()
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
		p.mu.Lock()
		p.queueClosed = true
		p.queueCond.Broadcast()
		p.mu.Unlock()
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
