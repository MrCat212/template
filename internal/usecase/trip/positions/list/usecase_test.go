package list_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/MrCat212/template/internal/domain/trip"
	"github.com/MrCat212/template/internal/usecase/trip/positions/list"
)

func TestListPositionsSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockpositionRepository(ctrl)

	tripID := uuid.New()

	expected := []trip.Position{
		{
			ID:         1,
			TripID:     tripID,
			Latitude:   55.75,
			Longitude:  37.61,
			RecordedAt: time.Now().UTC(),
		},
	}

	repo.EXPECT().
		ListPositions(gomock.Any(), tripID).
		Return(expected, nil)

	uc := list.New(repo)

	result, err := uc.Execute(context.Background(), tripID)
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 position, got %d", len(result))
	}

	if result[0].ID != expected[0].ID {
		t.Error("unexpected position ID")
	}
}

func TestListPositionsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMockpositionRepository(ctrl)

	tripID := uuid.New()

	repo.EXPECT().
		ListPositions(gomock.Any(), tripID).
		Return(nil, trip.ErrTripNotFound)

	uc := list.New(repo)

	_, err := uc.Execute(context.Background(), tripID)

	if !errors.Is(err, trip.ErrTripNotFound) {
		t.Errorf("expected ErrTripNotFound, got %v", err)
	}
}
