//go:build integration

package trip_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domain "github.com/MrCat212/template/internal/domain/trip"
	"github.com/MrCat212/template/internal/infra/postgres"
)

func TestPositionRepository(t *testing.T) {
	ctx := context.Background()

	pool, repo := setupRepository(t)
	tx := postgres.NewTxManager(pool, 3*time.Second)

	newTrip := makeTrip()
	cleanupTrip(t, pool, newTrip.ID)

	if err := repo.Create(ctx, newTrip); err != nil {
		t.Fatal(err)
	}

	base := time.Now().UTC().Truncate(time.Microsecond)

	newer := domain.Position{
		TripID:     newTrip.ID,
		Latitude:   55.75,
		Longitude:  37.61,
		RecordedAt: base,
	}

	older := domain.Position{
		TripID:     newTrip.ID,
		Latitude:   55.74,
		Longitude:  37.60,
		RecordedAt: base.Add(-5 * time.Minute),
	}

	for _, position := range []domain.Position{newer, older} {
		err := tx.Do(ctx, func(txCtx context.Context) error {
			saved, err := repo.SavePosition(txCtx, position)
			if err != nil {
				return err
			}

			return repo.UpdateLastPosition(
				txCtx,
				saved.TripID,
				saved.RecordedAt,
			)
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	positions, err := repo.ListPositions(ctx, newTrip.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(positions) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(positions))
	}

	if !positions[0].RecordedAt.Equal(older.RecordedAt) {
		t.Error("positions are not sorted by recorded_at")
	}

	if !positions[1].RecordedAt.Equal(newer.RecordedAt) {
		t.Error("newer position should be last")
	}

	var lastPositionAt time.Time

	err = pool.QueryRow(
		ctx,
		"SELECT last_position_at FROM trips WHERE id = $1",
		newTrip.ID,
	).Scan(&lastPositionAt)
	if err != nil {
		t.Fatal(err)
	}

	if !lastPositionAt.Equal(newer.RecordedAt) {
		t.Errorf(
			"last_position_at = %v, expected %v",
			lastPositionAt,
			newer.RecordedAt,
		)
	}
}

func TestPositionRollbackForCompletedTrip(t *testing.T) {
	ctx := context.Background()

	pool, repo := setupRepository(t)
	tx := postgres.NewTxManager(pool, 3*time.Second)

	newTrip := makeTrip()
	cleanupTrip(t, pool, newTrip.ID)

	if err := repo.Create(ctx, newTrip); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.Finish(ctx, newTrip.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	position := domain.Position{
		TripID:     newTrip.ID,
		Latitude:   55.75,
		Longitude:  37.61,
		RecordedAt: time.Now().UTC(),
	}

	err := tx.Do(ctx, func(txCtx context.Context) error {
		saved, err := repo.SavePosition(txCtx, position)
		if err != nil {
			return err
		}

		return repo.UpdateLastPosition(
			txCtx,
			saved.TripID,
			saved.RecordedAt,
		)
	})

	if !errors.Is(err, domain.ErrTripCompleted) {
		t.Fatalf("expected ErrTripCompleted, got %v", err)
	}

	positions, err := repo.ListPositions(ctx, newTrip.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(positions) != 0 {
		t.Errorf("expected rollback, got %d positions", len(positions))
	}
}

func TestListPositionsTripNotFound(t *testing.T) {
	ctx := context.Background()

	_, repo := setupRepository(t)

	_, err := repo.ListPositions(ctx, uuid.New())
	if !errors.Is(err, domain.ErrTripNotFound) {
		t.Errorf("expected ErrTripNotFound, got %v", err)
	}
}
