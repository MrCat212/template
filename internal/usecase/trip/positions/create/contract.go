//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package create

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/MrCat212/template/internal/domain/trip"
)

type positionRepository interface {
	SavePosition(ctx context.Context, position trip.Position) (trip.Position, error)
	UpdateLastPosition(ctx context.Context, tripID uuid.UUID, recordedAt time.Time) error
}

type txManager interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}
