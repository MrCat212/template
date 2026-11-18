package create_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/MrCat212/template/internal/domain/trip"
	"github.com/MrCat212/template/internal/usecase/trip/positions/create"
)

func TestCreatePositionSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMockpositionRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	input := create.Input{
		TripID:     uuid.New(),
		Latitude:   55.75,
		Longitude:  37.61,
		RecordedAt: time.Now().UTC(),
	}

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)

	repo.EXPECT().
		SavePosition(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, position trip.Position) (trip.Position, error) {
			position.ID = 42
			return position, nil
		})

	repo.EXPECT().
		UpdateLastPosition(gomock.Any(), input.TripID, input.RecordedAt).
		Return(nil)

	uc := create.New(repo, tx)

	result, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	if result.ID != 42 {
		t.Errorf("expected ID 42, got %d", result.ID)
	}
}

func TestCreatePositionValidation(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMockpositionRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	input := create.Input{
		TripID:     uuid.New(),
		Latitude:   100,
		Longitude:  37.61,
		RecordedAt: time.Now().UTC(),
	}

	uc := create.New(repo, tx)

	_, err := uc.Execute(context.Background(), input)

	var validationErr *trip.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCreatePositionUpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)

	repo := NewMockpositionRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	input := create.Input{
		TripID:     uuid.New(),
		Latitude:   55.75,
		Longitude:  37.61,
		RecordedAt: time.Now().UTC(),
	}

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)

	repo.EXPECT().
		SavePosition(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, position trip.Position) (trip.Position, error) {
			position.ID = 42
			return position, nil
		})

	repo.EXPECT().
		UpdateLastPosition(gomock.Any(), input.TripID, input.RecordedAt).
		Return(trip.ErrTripCompleted)

	uc := create.New(repo, tx)

	result, err := uc.Execute(context.Background(), input)

	if !errors.Is(err, trip.ErrTripCompleted) {
		t.Fatalf("expected ErrTripCompleted, got %v", err)
	}

	if result.ID != 0 {
		t.Error("expected empty position on error")
	}
}

type cancelSpy struct {
	called []uuid.UUID
}

func (s *cancelSpy) Cancel(id uuid.UUID) {
	s.called = append(s.called, id)
}

func TestCreatePositionCancellationAfterCommit(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockpositionRepository(ctrl)
	tx := NewMocktxManager(ctrl)
	spy := &cancelSpy{}
	input := create.Input{
		TripID: uuid.New(), Latitude: 55.75, Longitude: 37.61,
		RecordedAt: time.Now().UTC(),
	}

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
	)
	repo.EXPECT().SavePosition(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, position trip.Position) (trip.Position, error) {
			if len(spy.called) != 0 {
				t.Error("cancellation happened before the transaction finished")
			}
			position.ID = 13
			return position, nil
		},
	)
	repo.EXPECT().UpdateLastPosition(gomock.Any(), input.TripID, input.RecordedAt).Return(nil)

	result, err := create.New(repo, tx, spy).Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != 13 || len(spy.called) != 1 || spy.called[0] != input.TripID {
		t.Fatalf("unexpected saved position or cancellations: result=%+v, canceled=%v", result, spy.called)
	}
}

func TestCreatePositionDoesNotCancelOnRollback(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockpositionRepository(ctrl)
	tx := NewMocktxManager(ctrl)
	spy := &cancelSpy{}
	input := create.Input{TripID: uuid.New(), Latitude: 55.75, Longitude: 37.61, RecordedAt: time.Now().UTC()}

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).Return(errors.New("transaction failed"))

	_, err := create.New(repo, tx, spy).Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected transaction failure")
	}
	if len(spy.called) != 0 {
		t.Fatalf("canceled %d tasks for an unsuccessful transaction", len(spy.called))
	}
}
