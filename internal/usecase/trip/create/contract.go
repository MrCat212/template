//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package create

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/MrCat212/template/internal/domain/trip"
)

type tripRepository interface {
	Create(ctx context.Context, trip trip.Trip) error
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
