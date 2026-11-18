package create

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/MrCat212/template/internal/domain/trip"
)

type Input struct {
	TripID     uuid.UUID
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
}

type Usecase struct {
	repository positionRepository
	txManager  txManager
	canceler   positionCanceler
	metrics    positionMetrics
}

// New accepts an optional canceler so existing isolated usecase tests remain valid.
func New(repository positionRepository, txManager txManager, cancelers ...positionCanceler) *Usecase {
	u := &Usecase{repository: repository, txManager: txManager}
	if len(cancelers) > 0 {
		u.canceler = cancelers[0]
	}
	return u
}

// WithMetrics enables counting coordinates only after they are committed.
func (u *Usecase) WithMetrics(recorder positionMetrics) *Usecase {
	u.metrics = recorder
	return u
}

func (u *Usecase) Execute(ctx context.Context, input Input) (trip.Position, error) {
	position := trip.Position{
		TripID:     input.TripID,
		Latitude:   input.Latitude,
		Longitude:  input.Longitude,
		RecordedAt: input.RecordedAt,
	}

	if err := position.Validate(time.Now().UTC()); err != nil {
		return trip.Position{}, err
	}

	var saved trip.Position

	err := u.txManager.Do(ctx, func(txCtx context.Context) error {
		var err error

		saved, err = u.repository.SavePosition(txCtx, position)
		if err != nil {
			return fmt.Errorf("save position: %w", err)
		}

		if err := u.repository.UpdateLastPosition(
			txCtx,
			saved.TripID,
			saved.RecordedAt,
		); err != nil {
			return fmt.Errorf("update last position: %w", err)
		}

		return nil
	})
	if err != nil {
		return trip.Position{}, err
	}

	// Only cancel an outstanding push after the DB transaction commits.
	if u.canceler != nil {
		u.canceler.Cancel(saved.TripID)
	}
	if u.metrics != nil {
		u.metrics.PositionSaved(ctx)
	}

	return saved, nil
}
