//go:build integration

package trip_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	domain "github.com/MrCat212/template/internal/domain/trip"
)

func TestFindStaleTripsBatchKeyset(t *testing.T) {
	ctx := context.Background()
	pool, repo := setupRepository(t)
	marker := uuid.New()

	// All 250 trips have exactly the same timestamp. UUID breaks the tie.
	// A very old timestamp keeps unrelated recent trips out of the selection.
	startedAt := time.Now().UTC().Add(-365 * 24 * time.Hour).Truncate(time.Microsecond)
	const createSQL = `
		INSERT INTO trips (
			id, user_id, driver_id,
			start_latitude, start_longitude, end_latitude, end_longitude,
			price, status, started_at
		)
		SELECT gen_random_uuid(), $1, gen_random_uuid(),
			55.75, 37.61, 55.76, 37.62, 100, 'active', $2
		FROM generate_series(1, 250)
	`
	if _, err := pool.Exec(ctx, createSQL, marker, startedAt); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanupCtx, "DELETE FROM trips WHERE user_id = $1", marker); err != nil {
			t.Errorf("cleanup batch test trips: %v", err)
		}
	})

	var cursor *domain.StaleCursor
	seen := make(map[uuid.UUID]bool, 250)
	const limit = 100
	for page := range 3 {
		batch, err := repo.FindStaleTripsBatch(ctx, 180*24*time.Hour, limit, cursor)
		if err != nil {
			t.Fatal(err)
		}
		expected := limit
		if page == 2 {
			expected = 50
		}
		if len(batch) != expected {
			t.Fatalf("page %d: expected %d trips, got %d", page+1, expected, len(batch))
		}
		for i, item := range batch {
			if seen[item.ID] {
				t.Fatalf("duplicate trip across pages: %s", item.ID)
			}
			seen[item.ID] = true
			if !item.LastActivityAt.Equal(startedAt) {
				t.Errorf("unexpected last activity: %v", item.LastActivityAt)
			}
			if i > 0 && batch[i-1].ID.String() >= item.ID.String() {
				t.Fatal("equal-time records are not ordered by UUID")
			}
		}
		last := batch[len(batch)-1]
		cursor = &domain.StaleCursor{LastActivityAt: last.LastActivityAt, ID: last.ID}
	}
	if len(seen) != 250 {
		t.Fatalf("expected 250 distinct trips, got %d", len(seen))
	}

	empty, err := repo.FindStaleTripsBatch(ctx, 180*24*time.Hour, limit, cursor)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected end of scan, got %d records", len(empty))
	}
}

func TestFindStaleTripsBatchInvalidArguments(t *testing.T) {
	_, repo := setupRepository(t)
	ctx := context.Background()

	if _, err := repo.FindStaleTripsBatch(ctx, 0, 100, nil); err == nil {
		t.Fatal("expected duration validation error")
	}
	if _, err := repo.FindStaleTripsBatch(ctx, time.Minute, 0, nil); err == nil {
		t.Fatal("expected limit validation error")
	}
	if _, err := repo.FindStaleTripsBatch(ctx, time.Minute, 100, &domain.StaleCursor{}); err == nil {
		t.Fatal("expected invalid cursor error")
	}
}
