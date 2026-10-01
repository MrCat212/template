package trip

import (
	"context"
	"time"

	"github.com/MrCat212/template/internal/database"
	"github.com/google/uuid"
)

type CreateInput struct {
	UserID   uuid.UUID
	DriverID uuid.UUID

	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64

	Price int64
}

type Service struct {
	repository *Repository
	txManager  database.TxManager
}

func NewService(
	repository *Repository,
	txManager database.TxManager,
) *Service {
	return &Service{
		repository: repository,
		txManager:  txManager,
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Trip, error) {
	now := time.Now().UTC()

	trip := Trip{
		ID:       uuid.New(),
		UserID:   input.UserID,
		DriverID: input.DriverID,

		StartLatitude:  input.StartLatitude,
		StartLongitude: input.StartLongitude,
		EndLatitude:    input.EndLatitude,
		EndLongitude:   input.EndLongitude,

		Price:  input.Price,
		Status: StatusActive,

		StartedAt:  now,
		FinishedAt: nil,
	}

	err := s.txManager.Do(ctx, func(txCtx context.Context) error {
		if err := s.repository.Create(txCtx, trip); err != nil {
			return err
		}

		if err := s.repository.AddStatusHistory(
			txCtx,
			trip.ID,
			nil,
			StatusActive,
			"trip created",
			now,
		); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return Trip{}, err
	}

	return trip, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (Trip, error) {
	return s.repository.GetByID(ctx, id)
}

func (s *Service) Finish(ctx context.Context, id uuid.UUID) (Trip, error) {
	now := time.Now().UTC()

	var finishedTrip Trip

	err := s.txManager.Do(ctx, func(txCtx context.Context) error {
		trip, err := s.repository.Finish(txCtx, id, now)
		if err != nil {
			return err
		}

		fromStatus := StatusActive

		if err := s.repository.AddStatusHistory(
			txCtx,
			id,
			&fromStatus,
			StatusCompleted,
			"trip completed",
			now,
		); err != nil {
			return err
		}

		finishedTrip = trip

		return nil
	})
	if err != nil {
		return Trip{}, err
	}

	return finishedTrip, nil
}
