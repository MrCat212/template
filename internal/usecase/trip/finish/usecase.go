package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/MrCat212/template/internal/domain/trip"
)

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

func (u *Usecase) Execute(ctx context.Context, id uuid.UUID) (trip.Trip, error) {
	now := time.Now().UTC()

	var result trip.Trip

	err := u.txManager.Do(ctx, func(txCtx context.Context) error {
		currentTrip, err := u.repository.GetByID(txCtx, id)
		if err != nil {
			return err
		}

		previousStatus := currentTrip.Status

		if err := currentTrip.Finish(now); err != nil {
			return err
		}

		finishedTrip, err := u.repository.Finish(txCtx, id, now)
		if err != nil {
			return err
		}

		err = u.repository.AddStatusHistory(
			txCtx,
			id,
			&previousStatus,
			trip.StatusCompleted,
			"trip completed",
			now,
		)
		if err != nil {
			return err
		}

		result = finishedTrip

		return nil
	})
	if err != nil {
		return trip.Trip{}, err
	}

	return result, nil
}
