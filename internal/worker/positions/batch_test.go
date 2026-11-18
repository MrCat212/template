package positions

import (
	"container/heap"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/MrCat212/template/internal/domain/trip"
)

type batchFinderFunc func(context.Context, time.Duration, int, *trip.StaleCursor) ([]trip.StaleTrip, error)

func (f batchFinderFunc) FindStaleTripsBatch(ctx context.Context, d time.Duration, limit int, after *trip.StaleCursor) ([]trip.StaleTrip, error) {
	return f(ctx, d, limit, after)
}

func makeBatchTrips(t *testing.T, count int) []trip.StaleTrip {
	t.Helper()
	items := make([]trip.StaleTrip, 0, count)
	at := time.Now().UTC().Add(-time.Hour)
	for i := 1; i <= count; i++ {
		id := uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-%012d", i))
		items = append(items, trip.StaleTrip{ID: id, DriverID: uuid.New(), LastActivityAt: at})
	}
	return items
}

// scriptedBatchFinder has deterministic keyset semantics, including equal timestamps.
func scriptedBatchFinder(items []trip.StaleTrip, calls *int) batchFinderFunc {
	return func(_ context.Context, _ time.Duration, limit int, after *trip.StaleCursor) ([]trip.StaleTrip, error) {
		*calls++
		start := 0
		if after != nil {
			for start < len(items) && items[start].ID != after.ID {
				start++
			}
			if start == len(items) {
				return nil, fmt.Errorf("unknown cursor")
			}
			start++
		}
		end := min(start+limit, len(items))
		return items[start:end], nil
	}
}

func TestScannerReads250TripsInThreeBatches(t *testing.T) {
	cfg := testConfig()
	cfg.QueueSize = 300
	cfg.ScanBatchSize = 100

	items := makeBatchTrips(t, 250)
	calls := 0
	pool, err := New(scriptedBatchFinder(items, &calls), requesterFunc(noOpRequester), testLogger(), cfg, newManualTicker())
	if err != nil {
		t.Fatal(err)
	}
	pool.scanOnce(context.Background())
	if calls != 3 {
		t.Fatalf("expected 3 SQL batches, got %d", calls)
	}
	if got := len(pool.queue); got != 250 {
		t.Fatalf("expected all 250 jobs enqueued, got %d", got)
	}
	seen := make(map[uuid.UUID]bool, len(items))
	for pool.queue.Len() > 0 {
		job := heap.Pop(&pool.queue).(*queuedTask).job
		if seen[job.trip.ID] {
			t.Fatalf("duplicate queued trip: %s", job.trip.ID)
		}
		seen[job.trip.ID] = true
	}
	if len(seen) != len(items) {
		t.Fatalf("expected %d unique trips, got %d", len(items), len(seen))
	}
}

func TestScannerStopsFetchingOnFirstDroppedTask(t *testing.T) {
	cfg := testConfig()
	cfg.QueueSize = 50
	cfg.ScanBatchSize = 100

	items := makeBatchTrips(t, 250)
	calls := 0
	pool, err := New(scriptedBatchFinder(items, &calls), requesterFunc(noOpRequester), testLogger(), cfg, newManualTicker())
	if err != nil {
		t.Fatal(err)
	}
	pool.scanOnce(context.Background())
	if calls != 1 {
		t.Fatalf("expected only first SQL batch, got %d", calls)
	}
	if got := len(pool.queue); got != 50 {
		t.Fatalf("expected 50 queued jobs, got %d", got)
	}
	// The next tick starts from the oldest key rather than continuing the old cursor.
	pool.Cancel(items[0].ID)
	pool.scanOnce(context.Background())
	if calls != 2 {
		t.Fatalf("expected another first-page fetch on next tick, got %d", calls)
	}
}

func TestScannerRejectsInvalidBatchSize(t *testing.T) {
	cfg := testConfig()
	cfg.ScanBatchSize = 0
	if _, err := New(batchFinderFunc(func(context.Context, time.Duration, int, *trip.StaleCursor) ([]trip.StaleTrip, error) {
		return nil, nil
	}), requesterFunc(noOpRequester), testLogger(), cfg, newManualTicker()); err == nil {
		t.Fatal("expected invalid scan batch size error")
	}
}
