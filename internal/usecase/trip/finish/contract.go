//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/MrCat212/template/internal/domain/trip"
)

type tripRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (trip.Trip, error)
	Finish(ctx context.Context, id uuid.UUID, finishedAt time.Time) (trip.Trip, error)
	AddStatusHistory(
		ctx context.Context,
		tripID uuid.UUID,
		fromStatus *trip.Status,
		toStatus trip.Status,
		reason string,
		changedAt time.Time,
	) error
}

type txManager interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}
