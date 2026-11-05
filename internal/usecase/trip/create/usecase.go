package create

import (
	"context"
	"time"

	"github.com/MrCat212/template/internal/domain/trip"
	"github.com/google/uuid"
)

type Input struct {
	UserID   uuid.UUID
	DriverID uuid.UUID

	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64

	Price int64
}

type Usecase struct {
	repository tripRepository
	txManager  txManager
}

func New(repository tripRepository, txManager txManager) *Usecase {
	return &Usecase{
		repository: repository,
		txManager:  txManager,
	}
}

func (u *Usecase) Execute(ctx context.Context, input Input) (trip.Trip, error) {
	now := time.Now().UTC()

	newTrip := trip.Trip{
		ID:       uuid.New(),
		UserID:   input.UserID,
		DriverID: input.DriverID,

		StartLatitude:  input.StartLatitude,
		StartLongitude: input.StartLongitude,
		EndLatitude:    input.EndLatitude,
		EndLongitude:   input.EndLongitude,

		Price:  input.Price,
		Status: trip.StatusActive,

		StartedAt: now,
	}

	err := u.txManager.Do(ctx, func(txCtx context.Context) error {
		if err := u.repository.Create(txCtx, newTrip); err != nil {
			return err
		}

		return u.repository.AddStatusHistory(
			txCtx,
			newTrip.ID,
			nil,
			trip.StatusActive,
			"trip created",
			now,
		)
	})
	if err != nil {
		return trip.Trip{}, err
	}

	return newTrip, nil
}
