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

type positionSavedSpy struct {
	recorded int
}

func (s *positionSavedSpy) PositionSaved(context.Context) {
	s.recorded++
}

func TestPositionSavedMetricOnlyAfterCommit(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockpositionRepository(ctrl)
	tx := NewMocktxManager(ctrl)
	spy := &positionSavedSpy{}
	input := create.Input{
		TripID: uuid.New(), Latitude: 55.75, Longitude: 37.61,
		RecordedAt: time.Now().UTC(),
	}

	tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			if err := fn(ctx); err != nil {
				return err
			}
			if spy.recorded != 0 {
				t.Fatal("recorded saved metric before commit")
			}
			return nil
		},
	)
	repo.EXPECT().SavePosition(gomock.Any(), gomock.Any()).Return(trip.Position{
		ID: 42, TripID: input.TripID, RecordedAt: input.RecordedAt,
	}, nil)
	repo.EXPECT().UpdateLastPosition(gomock.Any(), input.TripID, input.RecordedAt).Return(nil)

	result, err := create.New(repo, tx).WithMetrics(spy).Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != 42 || spy.recorded != 1 {
		t.Fatalf("wrong result or saved metric count: %+v, %d", result, spy.recorded)
	}
}

func TestPositionSavedMetricNotRecordedOnRollback(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockpositionRepository(ctrl)
	tx := NewMocktxManager(ctrl)
	spy := &positionSavedSpy{}
	input := create.Input{
		TripID: uuid.New(), Latitude: 55.75, Longitude: 37.61,
		RecordedAt: time.Now().UTC(),
	}
	tx.EXPECT().Do(gomock.Any(), gomock.Any()).Return(errors.New("transaction failed"))

	_, err := create.New(repo, tx).WithMetrics(spy).Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected failed transaction")
	}
	if spy.recorded != 0 {
		t.Fatalf("counted %d uncommitted positions", spy.recorded)
	}
}
