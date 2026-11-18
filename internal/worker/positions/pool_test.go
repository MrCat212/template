package positions

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/MrCat212/template/internal/domain/trip"
)

type finderFunc func(context.Context, time.Duration) ([]trip.StaleTrip, error)

func (f finderFunc) FindStaleTrips(ctx context.Context, d time.Duration) ([]trip.StaleTrip, error) {
	return f(ctx, d)
}

type requesterFunc func(context.Context, trip.StaleTrip) error

func (f requesterFunc) RequestPosition(ctx context.Context, t trip.StaleTrip) error { return f(ctx, t) }

type manualTicker struct {
	ticks   chan time.Time
	stopped chan struct{}
	once    sync.Once
}

func newManualTicker() *manualTicker {
	return &manualTicker{ticks: make(chan time.Time), stopped: make(chan struct{})}
}
func (t *manualTicker) Chan() <-chan time.Time { return t.ticks }
func (t *manualTicker) Stop()                  { t.once.Do(func() { close(t.stopped) }) }

func testConfig() Config {
	return Config{CheckInterval: time.Hour, StaleAfter: time.Minute, Workers: 1, QueueSize: 2, RequestTimeout: time.Second}
}

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func emptyFinder(context.Context, time.Duration) ([]trip.StaleTrip, error) { return nil, nil }

func noOpRequester(context.Context, trip.StaleTrip) error { return nil }

func testStop(t *testing.T, p *Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		t.Fatalf("stop pool: %v", err)
	}
}

func waitID(t *testing.T, ch <-chan uuid.UUID) uuid.UUID {
	t.Helper()
	select {
	case id := <-ch:
		return id
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for worker")
		return uuid.Nil
	}
}

func TestScannerUsesManualTicker(t *testing.T) {
	clock := newManualTicker()
	calls := make(chan time.Duration, 1)
	p, err := New(finderFunc(func(_ context.Context, d time.Duration) ([]trip.StaleTrip, error) {
		calls <- d
		return nil, nil
	}), requesterFunc(noOpRequester), testLogger(), testConfig(), clock)
	if err != nil {
		t.Fatal(err)
	}
	p.Start()
	defer testStop(t, p)
	clock.ticks <- time.Now()
	select {
	case got := <-calls:
		if got != time.Minute {
			t.Errorf("stale after = %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("scanner did not run")
	}
}

func TestQueuedDuplicateNotSentTwice(t *testing.T) {
	clock := newManualTicker()
	started := make(chan uuid.UUID, 2)
	release := make(chan struct{})
	item := trip.StaleTrip{ID: uuid.New(), DriverID: uuid.New()}
	p, err := New(finderFunc(emptyFinder), requesterFunc(func(ctx context.Context, t trip.StaleTrip) error {
		started <- t.ID
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}), testLogger(), testConfig(), clock)
	if err != nil {
		t.Fatal(err)
	}
	p.Start()
	if result := p.enqueue(context.Background(), item); result != resultQueued {
		t.Fatal(result)
	}
	if got := waitID(t, started); got != item.ID {
		t.Fatal("wrong trip")
	}
	if result := p.enqueue(context.Background(), item); result != resultDuplicate {
		t.Fatalf("expected duplicate, got %s", result)
	}
	close(release)
	testStop(t, p)
	select {
	case <-started:
		t.Fatal("duplicate was sent")
	default:
	}
}

func TestCancelQueuedTask(t *testing.T) {
	started := make(chan uuid.UUID, 2)
	release := make(chan struct{})
	first := trip.StaleTrip{ID: uuid.New(), DriverID: uuid.New()}
	second := trip.StaleTrip{ID: uuid.New(), DriverID: uuid.New()}
	p, err := New(finderFunc(emptyFinder), requesterFunc(func(ctx context.Context, item trip.StaleTrip) error {
		started <- item.ID
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}), testLogger(), testConfig(), newManualTicker())
	if err != nil {
		t.Fatal(err)
	}
	p.Start()
	if got := p.enqueue(context.Background(), first); got != resultQueued {
		t.Fatal(got)
	}
	if got := waitID(t, started); got != first.ID {
		t.Fatal("unexpected first request")
	}
	if got := p.enqueue(context.Background(), second); got != resultQueued {
		t.Fatal(got)
	}
	p.Cancel(second.ID)
	close(release)
	testStop(t, p)
	select {
	case <-started:
		t.Fatal("canceled task was sent")
	default:
	}
}

func TestQueueFullDrops(t *testing.T) {
	cfg := testConfig()
	cfg.QueueSize = 1
	p, err := New(finderFunc(emptyFinder), requesterFunc(noOpRequester), testLogger(), cfg, newManualTicker())
	if err != nil {
		t.Fatal(err)
	}
	first := trip.StaleTrip{ID: uuid.New()}
	second := trip.StaleTrip{ID: uuid.New()}
	if got := p.enqueue(context.Background(), first); got != resultQueued {
		t.Fatal(got)
	}
	if got := p.enqueue(context.Background(), second); got != resultDropped {
		t.Fatalf("expected dropped, got %s", got)
	}
	testStop(t, p)
}

func TestFixedNumberOfWorkers(t *testing.T) {
	cfg := testConfig()
	cfg.Workers = 3
	cfg.QueueSize = 3
	started := make(chan uuid.UUID, cfg.Workers+1)
	release := make(chan struct{})
	var total atomic.Int32
	p, err := New(finderFunc(emptyFinder), requesterFunc(func(ctx context.Context, item trip.StaleTrip) error {
		total.Add(1)
		started <- item.ID
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}), testLogger(), cfg, newManualTicker())
	if err != nil {
		t.Fatal(err)
	}
	p.Start()
	for range cfg.Workers {
		if got := p.enqueue(context.Background(), trip.StaleTrip{ID: uuid.New()}); got != resultQueued {
			t.Fatal(got)
		}
		waitID(t, started)
	}
	if got := p.enqueue(context.Background(), trip.StaleTrip{ID: uuid.New()}); got != resultQueued {
		t.Fatal(got)
	}
	if n := int(total.Load()); n != cfg.Workers {
		t.Fatalf("expected %d active workers, got %d", cfg.Workers, n)
	}
	close(release)
	testStop(t, p)
	if n := int(total.Load()); n != cfg.Workers+1 {
		t.Errorf("expected all tasks processed, got %d", n)
	}
}

func TestSenderErrorDoesNotKillWorker(t *testing.T) {
	var calls atomic.Int32
	started := make(chan uuid.UUID, 2)
	p, err := New(finderFunc(emptyFinder), requesterFunc(func(_ context.Context, item trip.StaleTrip) error {
		n := calls.Add(1)
		started <- item.ID
		if n == 1 {
			return errors.New("push unavailable")
		}
		return nil
	}), testLogger(), testConfig(), newManualTicker())
	if err != nil {
		t.Fatal(err)
	}
	p.Start()
	for range 2 {
		if got := p.enqueue(context.Background(), trip.StaleTrip{ID: uuid.New()}); got != resultQueued {
			t.Fatal(got)
		}
		waitID(t, started)
	}
	testStop(t, p)
	if got := calls.Load(); got != 2 {
		t.Errorf("expected 2 requests, got %d", got)
	}
}

func TestCancelRunningRequest(t *testing.T) {
	started := make(chan uuid.UUID, 1)
	canceled := make(chan struct{})
	p, err := New(finderFunc(emptyFinder), requesterFunc(func(ctx context.Context, item trip.StaleTrip) error {
		started <- item.ID
		<-ctx.Done()
		close(canceled)
		return ctx.Err()
	}), testLogger(), testConfig(), newManualTicker())
	if err != nil {
		t.Fatal(err)
	}
	p.Start()
	id := uuid.New()
	if got := p.enqueue(context.Background(), trip.StaleTrip{ID: id}); got != resultQueued {
		t.Fatal(got)
	}
	waitID(t, started)
	p.Cancel(id)
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("running request was not canceled")
	}
	testStop(t, p)
}

func TestStopCancelsSlowRequest(t *testing.T) {
	started := make(chan uuid.UUID, 1)
	p, err := New(finderFunc(emptyFinder), requesterFunc(func(ctx context.Context, item trip.StaleTrip) error {
		started <- item.ID
		<-ctx.Done()
		return ctx.Err()
	}), testLogger(), testConfig(), newManualTicker())
	if err != nil {
		t.Fatal(err)
	}
	p.Start()
	if got := p.enqueue(context.Background(), trip.StaleTrip{ID: uuid.New()}); got != resultQueued {
		t.Fatal(got)
	}
	waitID(t, started)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := p.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}
