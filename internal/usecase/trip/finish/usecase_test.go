package finish_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MrCat212/template/internal/domain/trip"
	"github.com/MrCat212/template/internal/usecase/trip/finish"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func newTestTx(ctrl *gomock.Controller) *MocktxManager {
	tx := NewMocktxManager(ctrl)

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)

	return tx
}

func TestFinishSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	tx := newTestTx(ctrl)

	id := uuid.New()

	repo.EXPECT().
		GetByID(gomock.Any(), id).
		Return(trip.Trip{ID: id, Status: trip.StatusActive}, nil)

	repo.EXPECT().
		Finish(gomock.Any(), id, gomock.Any()).
		DoAndReturn(
			func(ctx context.Context, tripID uuid.UUID, at time.Time) (trip.Trip, error) {
				return trip.Trip{
					ID:         tripID,
					Status:     trip.StatusCompleted,
					FinishedAt: &at,
				}, nil
			},
		)

	previousStatus := trip.StatusActive

	repo.EXPECT().
		AddStatusHistory(
			gomock.Any(),
			id,
			gomock.Eq(&previousStatus),
			trip.StatusCompleted,
			"trip completed",
			gomock.Any(),
		).
		Return(nil)

	u := finish.New(repo, tx)

	result, err := u.Execute(context.Background(), id)

	if err != nil {
		t.Fatalf("Не ожидали ошибку: %v", err)
	}

	if result.ID != id {
		t.Error("Неправильный ID поездки")
	}

	if result.Status != trip.StatusCompleted {
		t.Errorf("Ожидали completed, получили %s", result.Status)
	}

	if result.FinishedAt == nil {
		t.Error("Время завершения не записано")
	}
}

func TestFinishAlreadyCompleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	tx := newTestTx(ctrl)

	id := uuid.New()

	repo.EXPECT().
		GetByID(gomock.Any(), id).
		Return(trip.Trip{
			ID:     id,
			Status: trip.StatusCompleted,
		}, nil)

	u := finish.New(repo, tx)

	result, err := u.Execute(context.Background(), id)

	if !errors.Is(err, trip.ErrTripCompleted) {
		t.Errorf("Ожидали ErrTripCompleted, получили %v", err)
	}

	if result.ID != uuid.Nil {
		t.Error("При ошибке должна возвращаться пустая поездка")
	}
}

func TestFinishNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	tx := newTestTx(ctrl)

	id := uuid.New()

	repo.EXPECT().
		GetByID(gomock.Any(), id).
		Return(trip.Trip{}, trip.ErrTripNotFound)

	u := finish.New(repo, tx)

	result, err := u.Execute(context.Background(), id)

	if !errors.Is(err, trip.ErrTripNotFound) {
		t.Errorf("Ожидали ErrTripNotFound, получили %v", err)
	}

	if result.ID != uuid.Nil {
		t.Error("При ошибке должна возвращаться пустая поездка")
	}
}

func TestFinishConflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	tx := newTestTx(ctrl)

	id := uuid.New()

	repo.EXPECT().
		GetByID(gomock.Any(), id).
		Return(trip.Trip{
			ID:     id,
			Status: trip.StatusActive,
		}, nil)

	repo.EXPECT().
		Finish(gomock.Any(), id, gomock.Any()).
		Return(trip.Trip{}, trip.ErrTripCompleted)

	u := finish.New(repo, tx)

	result, err := u.Execute(context.Background(), id)

	if !errors.Is(err, trip.ErrTripCompleted) {
		t.Errorf("Ожидали ErrTripCompleted, получили %v", err)
	}

	if result.ID != uuid.Nil {
		t.Error("При ошибке должна возвращаться пустая поездка")
	}
}

func TestFinishHistoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	tx := newTestTx(ctrl)

	id := uuid.New()
	historyErr := errors.New("history error")

	repo.EXPECT().
		GetByID(gomock.Any(), id).
		Return(trip.Trip{
			ID:     id,
			Status: trip.StatusActive,
		}, nil)

	repo.EXPECT().
		Finish(gomock.Any(), id, gomock.Any()).
		Return(trip.Trip{
			ID:     id,
			Status: trip.StatusCompleted,
		}, nil)

	repo.EXPECT().
		AddStatusHistory(
			gomock.Any(),
			id,
			gomock.Any(),
			trip.StatusCompleted,
			"trip completed",
			gomock.Any(),
		).
		Return(historyErr)

	u := finish.New(repo, tx)

	result, err := u.Execute(context.Background(), id)

	if !errors.Is(err, historyErr) {
		t.Errorf("Ожидали ошибку %v, получили %v", historyErr, err)
	}

	if result.ID != uuid.Nil {
		t.Error("При ошибке должна возвращаться пустая поездка")
	}
}
