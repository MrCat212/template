package trip

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/MrCat212/template/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCreateRollback(t *testing.T) {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	repo := NewRepository(pool, 3*time.Second)
	txManager := database.NewTxManager(pool, 3*time.Second)

	now := time.Now().UTC()

	newTrip := Trip{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  55.75,
		StartLongitude: 37.61,
		EndLatitude:    55.76,
		EndLongitude:   37.62,
		Price:          500,
		Status:         StatusActive,
		StartedAt:      now,
	}

	err = txManager.Do(ctx, func(txCtx context.Context) error {
		if err := repo.Create(txCtx, newTrip); err != nil {
			return err
		}

		return repo.AddStatusHistory(
			txCtx, newTrip.ID, nil, Status("wrong"), "test", now,
		)
	})

	if err == nil {
		t.Fatal("expected an error")
	}

	_, err = repo.GetByID(ctx, newTrip.ID)
	if !errors.Is(err, ErrTripNotFound) {
		t.Fatalf("trip was not rolled back: %v", err)
	}
}

func TestBeginTransactionTimeout(t *testing.T) {
	ctx := context.Background()
	poolConfig, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	poolConfig.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	held, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()

	manager := database.NewTxManager(pool, 150*time.Millisecond)
	started := time.Now()
	err = manager.Do(ctx, func(context.Context) error {
		t.Error("transaction body must not start while pool is busy")
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("begin transaction took too long: %s", elapsed)
	}
}
