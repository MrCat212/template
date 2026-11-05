//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package get

import (
	"context"

	"github.com/MrCat212/template/internal/domain/trip"
	"github.com/google/uuid"
)

type tripRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (trip.Trip, error)
}
