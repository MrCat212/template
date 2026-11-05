package get_test

import (
	"context"
	"errors"
	"testing"

	"github.com/MrCat212/template/internal/domain/trip"
	"github.com/MrCat212/template/internal/usecase/trip/get"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestGetSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMocktripRepository(ctrl)

	tripID := uuid.New()

	expectedTrip := trip.Trip{
		ID:     tripID,
		Status: trip.StatusActive,
		Price:  500,
	}

	repo.EXPECT().
		GetByID(gomock.Any(), tripID).
		Return(expectedTrip, nil)

	usecase := get.New(repo)

	result, err := usecase.Execute(context.Background(), tripID)

	if err != nil {
		t.Fatalf("Не ожидали ошибку: %v", err)
	}

	if result.ID != expectedTrip.ID {
		t.Error("Получили неправильный ID поездки")
	}

	if result.Status != expectedTrip.Status {
		t.Errorf("Ожидали статус %s, получили %s", expectedTrip.Status, result.Status)
	}

	if result.Price != expectedTrip.Price {
		t.Errorf("Ожидали цену %d, получили %d", expectedTrip.Price, result.Price)
	}
}
func TestGetNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMocktripRepository(ctrl)

	tripID := uuid.New()

	repo.EXPECT().
		GetByID(gomock.Any(), tripID).
		Return(trip.Trip{}, trip.ErrTripNotFound)

	usecase := get.New(repo)

	result, err := usecase.Execute(context.Background(), tripID)

	if !errors.Is(err, trip.ErrTripNotFound) {
		t.Errorf("Ожидали ErrTripNotFound, получили %v", err)
	}

	if result.ID != uuid.Nil {
		t.Error("При ошибке должна возвращаться пустая поездка")
	}
}

func TestGetRepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMocktripRepository(ctrl)

	tripID := uuid.New()
	repoErr := errors.New("database error")

	repo.EXPECT().
		GetByID(gomock.Any(), tripID).
		Return(trip.Trip{}, repoErr)

	usecase := get.New(repo)

	result, err := usecase.Execute(context.Background(), tripID)

	if !errors.Is(err, repoErr) {
		t.Errorf("Ожидали ошибку %v, получили %v", repoErr, err)
	}

	if result.ID != uuid.Nil {
		t.Error("При ошибке должна возвращаться пустая поездка")
	}
}
