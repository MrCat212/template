package create_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/MrCat212/template/internal/domain/trip"
	"github.com/MrCat212/template/internal/usecase/trip/create"
)

func TestCreateSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	input := create.Input{
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  55.0,
		StartLongitude: 37.0,
		EndLatitude:    56.0,
		EndLongitude:   38.0,
		Price:          500,
	}

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	repo.EXPECT().AddStatusHistory(
		gomock.Any(),
		gomock.Any(),
		nil,
		trip.StatusActive,
		"trip created",
		gomock.Any(),
	).Return(nil)

	usecase := create.New(repo, tx)

	result, err := usecase.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Не ожидали ошибку: %v", err)
	}

	if result.Status != trip.StatusActive {
		t.Errorf("Ожидали статус active, получили %s", result.Status)
	}

	if result.UserID != input.UserID {
		t.Error("Неправильный ID пользователя")
	}
}

func TestCreateDriverBusy(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	input := create.Input{
		UserID:   uuid.New(),
		DriverID: uuid.New(),
		Price:    500,
	}

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(trip.ErrDriverBusy)
	repo.EXPECT().AddStatusHistory(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
	).Times(0)

	usecase := create.New(repo, tx)

	_, err := usecase.Execute(context.Background(), input)

	if !errors.Is(err, trip.ErrDriverBusy) {
		t.Errorf("Ожидали ErrDriverBusy, получили %v", err)
	}
}

func TestCreateHistoryError(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	input := create.Input{
		UserID:   uuid.New(),
		DriverID: uuid.New(),
		Price:    500,
	}

	historyErr := errors.New("history error")

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	repo.EXPECT().AddStatusHistory(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
	).Return(historyErr)

	usecase := create.New(repo, tx)

	result, err := usecase.Execute(context.Background(), input)

	if !errors.Is(err, historyErr) {
		t.Errorf("Ожидали ошибку %v, получили %v", historyErr, err)
	}

	if result.ID != uuid.Nil {
		t.Error("При ошибке должна возвращаться пустая поездка")
	}
}

func TestCreateRepositoryError(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	input := create.Input{
		UserID:   uuid.New(),
		DriverID: uuid.New(),
		Price:    500,
	}

	repoErr := errors.New("database error")

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(repoErr)

	usecase := create.New(repo, tx)

	result, err := usecase.Execute(context.Background(), input)

	if !errors.Is(err, repoErr) {
		t.Errorf("Ожидали ошибку %v, получили %v", repoErr, err)
	}

	if result.ID != uuid.Nil {
		t.Error("При ошибке должна возвращаться пустая поездка")
	}
}
