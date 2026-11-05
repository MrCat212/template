//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "github.com/MrCat212/template/internal/domain/trip"
	"github.com/MrCat212/template/internal/infra/postgres"
	repository "github.com/MrCat212/template/internal/repository/trip"
)

func setupRepository(t *testing.T) (*pgxpool.Pool, *repository.Repository) {
	t.Helper()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	return pool, repository.NewRepository(pool, 3*time.Second)
}

func makeTrip() domain.Trip {
	return domain.Trip{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  55.75,
		StartLongitude: 37.61,
		EndLatitude:    55.76,
		EndLongitude:   37.62,
		Price:          500,
		Status:         domain.StatusActive,
		StartedAt:      time.Now().UTC(),
	}
}

func cleanupTrip(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		_, err := pool.Exec(
			context.Background(),
			"DELETE FROM trips WHERE id = $1",
			id,
		)
		if err != nil {
			t.Errorf("cleanup trip: %v", err)
		}
	})
}

func TestRepositoryLifecycle(t *testing.T) {
	ctx := context.Background()

	pool, repo := setupRepository(t)
	tx := postgres.NewTxManager(pool)

	newTrip := makeTrip()
	cleanupTrip(t, pool, newTrip.ID)

	err := tx.Do(ctx, func(txCtx context.Context) error {
		if err := repo.Create(txCtx, newTrip); err != nil {
			return err
		}

		return repo.AddStatusHistory(
			txCtx,
			newTrip.ID,
			nil,
			domain.StatusActive,
			"trip created",
			newTrip.StartedAt,
		)
	})
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}

	created, err := repo.GetByID(ctx, newTrip.ID)
	if err != nil {
		t.Fatal(err)
	}

	if created.ID != newTrip.ID {
		t.Error("Неправильный ID поездки")
	}

	if created.Status != domain.StatusActive {
		t.Errorf("Ожидали active, получили %s", created.Status)
	}

	now := time.Now().UTC()

	err = tx.Do(ctx, func(txCtx context.Context) error {
		current, err := repo.GetByID(txCtx, newTrip.ID)
		if err != nil {
			return err
		}

		if err := current.Finish(now); err != nil {
			return err
		}

		if _, err := repo.Finish(txCtx, newTrip.ID, now); err != nil {
			return err
		}

		fromStatus := domain.StatusActive

		return repo.AddStatusHistory(
			txCtx,
			newTrip.ID,
			&fromStatus,
			domain.StatusCompleted,
			"trip completed",
			now,
		)
	})
	if err != nil {
		t.Fatalf("finish trip: %v", err)
	}

	finished, err := repo.GetByID(ctx, newTrip.ID)
	if err != nil {
		t.Fatal(err)
	}

	if finished.Status != domain.StatusCompleted {
		t.Errorf("Ожидали completed, получили %s", finished.Status)
	}

	if finished.FinishedAt == nil {
		t.Error("Время завершения не записано")
	}

	var historyCount int

	err = pool.QueryRow(
		ctx,
		"SELECT COUNT(*) FROM trip_status_history WHERE trip_id = $1",
		newTrip.ID,
	).Scan(&historyCount)
	if err != nil {
		t.Fatal(err)
	}

	if historyCount != 2 {
		t.Errorf("Ожидали 2 записи истории, получили %d", historyCount)
	}

	_, err = repo.Finish(ctx, newTrip.ID, time.Now().UTC())

	if !errors.Is(err, domain.ErrTripCompleted) {
		t.Errorf("Ожидали ErrTripCompleted, получили %v", err)
	}
}

func TestRepositoryDriverBusy(t *testing.T) {
	ctx := context.Background()

	pool, repo := setupRepository(t)

	first := makeTrip()
	second := makeTrip()

	second.DriverID = first.DriverID

	cleanupTrip(t, pool, first.ID)
	cleanupTrip(t, pool, second.ID)

	if err := repo.Create(ctx, first); err != nil {
		t.Fatal(err)
	}

	err := repo.Create(ctx, second)

	if !errors.Is(err, domain.ErrDriverBusy) {
		t.Errorf("Ожидали ErrDriverBusy, получили %v", err)
	}
}

func TestRepositoryRollback(t *testing.T) {
	ctx := context.Background()

	pool, repo := setupRepository(t)
	tx := postgres.NewTxManager(pool)

	newTrip := makeTrip()
	cleanupTrip(t, pool, newTrip.ID)

	err := tx.Do(ctx, func(txCtx context.Context) error {
		if err := repo.Create(txCtx, newTrip); err != nil {
			return err
		}

		return repo.AddStatusHistory(
			txCtx,
			newTrip.ID,
			nil,
			domain.Status("wrong"),
			"test",
			time.Now().UTC(),
		)
	})

	if err == nil {
		t.Fatal("Ожидали ошибку записи истории")
	}

	_, err = repo.GetByID(ctx, newTrip.ID)

	if !errors.Is(err, domain.ErrTripNotFound) {
		t.Errorf("Поездка не откатилась: %v", err)
	}
}

func TestRepositoryNotFound(t *testing.T) {
	ctx := context.Background()

	_, repo := setupRepository(t)

	id := uuid.New()

	_, err := repo.GetByID(ctx, id)

	if !errors.Is(err, domain.ErrTripNotFound) {
		t.Errorf("Ожидали ErrTripNotFound, получили %v", err)
	}

	_, err = repo.Finish(ctx, id, time.Now().UTC())

	if !errors.Is(err, domain.ErrTripNotFound) {
		t.Errorf("Ожидали ErrTripNotFound, получили %v", err)
	}
}
