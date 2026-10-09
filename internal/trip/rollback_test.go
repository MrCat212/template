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
	txManager := database.NewTxManager(pool)

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
