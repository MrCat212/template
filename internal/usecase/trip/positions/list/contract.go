//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/MrCat212/template/internal/domain/trip"
)

type positionRepository interface {
	ListPositions(ctx context.Context, tripID uuid.UUID) ([]trip.Position, error)
}
