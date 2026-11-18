package positions

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/MrCat212/template/internal/domain/trip"
)

func priorityTrip(n int, last time.Time) trip.StaleTrip {
	ids := [...]uuid.UUID{
		uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		uuid.MustParse("00000000-0000-0000-0000-000000000003"),
	}
	return trip.StaleTrip{ID: ids[n-1], DriverID: uuid.New(), LastActivityAt: last}
}

func TestPriorityOldestFirst(t *testing.T) {
	cfg := testConfig()
	cfg.QueueSize = 3
	base := time.Now().Add(-time.Hour)
	newer := priorityTrip(1, base.Add(20*time.Minute))
	older := priorityTrip(2, base)
	middle := priorityTrip(3, base.Add(10*time.Minute))
	started := make(chan uuid.UUID, 3)
	p, err := New(finderFunc(emptyFinder), requesterFunc(func(_ context.Context, item trip.StaleTrip) error {
		started <- item.ID
		return nil
	}), testLogger(), cfg, newManualTicker())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []trip.StaleTrip{newer, older, middle} {
		if got := p.enqueue(context.Background(), item); got != resultQueued {
			t.Fatal(got)
		}
	}
	p.Start()
	for _, want := range []uuid.UUID{older.ID, middle.ID, newer.ID} {
		if got := waitID(t, started); got != want {
			t.Fatalf("expected priority order %s, got %s", want, got)
		}
	}
	testStop(t, p)
}

func TestPriorityEvictsNewest(t *testing.T) {
	cfg := testConfig()
	cfg.QueueSize = 2
	base := time.Now().Add(-time.Hour)
	newest := priorityTrip(1, base.Add(20*time.Minute))
	middle := priorityTrip(2, base.Add(10*time.Minute))
	oldest := priorityTrip(3, base)
	p, err := New(finderFunc(emptyFinder), requesterFunc(noOpRequester), testLogger(), cfg, newManualTicker())
	if err != nil {
		t.Fatal(err)
	}
	if got := p.enqueue(context.Background(), middle); got != resultQueued {
		t.Fatal(got)
	}
	if got := p.enqueue(context.Background(), newest); got != resultQueued {
		t.Fatal(got)
	}
	if got := p.enqueue(context.Background(), oldest); got != resultReplaced {
		t.Fatalf("expected replacement, got %s", got)
	}
	if p.queue.Len() != cfg.QueueSize {
		t.Fatal("replacement changed queue size")
	}
	if _, ok := p.pending[newest.ID]; ok {
		t.Fatal("newest must be evicted")
	}
	for _, id := range []uuid.UUID{middle.ID, oldest.ID} {
		if _, ok := p.pending[id]; !ok {
			t.Fatalf("missing kept trip %s", id)
		}
	}
	testStop(t, p)
}

func TestPriorityRejectsNewer(t *testing.T) {
	cfg := testConfig()
	cfg.QueueSize = 1
	base := time.Now().Add(-time.Hour)
	oldest := priorityTrip(1, base)
	newer := priorityTrip(2, base.Add(time.Minute))
	p, err := New(finderFunc(emptyFinder), requesterFunc(noOpRequester), testLogger(), cfg, newManualTicker())
	if err != nil {
		t.Fatal(err)
	}
	if got := p.enqueue(context.Background(), oldest); got != resultQueued {
		t.Fatal(got)
	}
	if got := p.enqueue(context.Background(), newer); got != resultDropped {
		t.Fatalf("expected new task dropped, got %s", got)
	}
	if _, ok := p.pending[oldest.ID]; !ok {
		t.Fatal("oldest was evicted")
	}
	testStop(t, p)
}

func TestPriorityCancelPhysicallyRemovesTask(t *testing.T) {
	cfg := testConfig()
	cfg.QueueSize = 1
	base := time.Now().Add(-time.Hour)
	old := priorityTrip(1, base)
	newer := priorityTrip(2, base.Add(time.Minute))
	called := make(chan uuid.UUID, 1)
	p, err := New(finderFunc(emptyFinder), requesterFunc(func(_ context.Context, item trip.StaleTrip) error {
		called <- item.ID
		return nil
	}), testLogger(), cfg, newManualTicker())
	if err != nil {
		t.Fatal(err)
	}
	if got := p.enqueue(context.Background(), old); got != resultQueued {
		t.Fatal(got)
	}
	p.Cancel(old.ID)
	if p.queue.Len() != 0 {
		t.Fatal("cancelled job must be removed from heap")
	}
	if got := p.enqueue(context.Background(), newer); got != resultQueued {
		t.Fatal(got)
	}
	p.Start()
	if got := waitID(t, called); got != newer.ID {
		t.Fatalf("cancelled trip sent; got %s", got)
	}
	testStop(t, p)
}

func TestPriorityEqualTimestampUsesUUID(t *testing.T) {
	cfg := testConfig()
	cfg.QueueSize = 2
	base := time.Now().Add(-time.Hour)
	a := priorityTrip(1, base)
	b := priorityTrip(2, base)
	started := make(chan uuid.UUID, 2)
	p, err := New(finderFunc(emptyFinder), requesterFunc(func(_ context.Context, item trip.StaleTrip) error {
		started <- item.ID
		return nil
	}), testLogger(), cfg, newManualTicker())
	if err != nil {
		t.Fatal(err)
	}
	if got := p.enqueue(context.Background(), b); got != resultQueued {
		t.Fatal(got)
	}
	if got := p.enqueue(context.Background(), a); got != resultQueued {
		t.Fatal(got)
	}
	p.Start()
	if got := waitID(t, started); got != a.ID {
		t.Fatalf("expected lower UUID first, got %s", got)
	}
	if got := waitID(t, started); got != b.ID {
		t.Fatalf("expected higher UUID second, got %s", got)
	}
	testStop(t, p)
}

func TestPriorityScannerStopsAfterEviction(t *testing.T) {
	cfg := testConfig()
	cfg.QueueSize = 2
	cfg.ScanBatchSize = 3
	base := time.Now().Add(-time.Hour)
	newest := priorityTrip(1, base.Add(20*time.Minute))
	middle := priorityTrip(2, base.Add(10*time.Minute))
	oldest := priorityTrip(3, base)
	var calls int
	finder := batchFinderFunc(func(_ context.Context, _ time.Duration, _ int, _ *trip.StaleCursor) ([]trip.StaleTrip, error) {
		calls++
		if calls > 1 {
			t.Fatal("scanner fetched next batch after eviction")
		}
		return []trip.StaleTrip{oldest, middle, newest}, nil
	})
	p, err := New(finder, requesterFunc(noOpRequester), testLogger(), cfg, newManualTicker())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []trip.StaleTrip{middle, newest} {
		if got := p.enqueue(context.Background(), item); got != resultQueued {
			t.Fatal(got)
		}
	}
	p.scanOnce(context.Background())
	if calls != 1 {
		t.Fatalf("expected 1 SQL call, got %d", calls)
	}
	if _, ok := p.pending[oldest.ID]; !ok {
		t.Fatal("oldest trip was not inserted")
	}
	if _, ok := p.pending[newest.ID]; ok {
		t.Fatal("newest trip should have been evicted")
	}
	testStop(t, p)
}
