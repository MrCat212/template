//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package finish

import (
	"context"

	domain "github.com/MrCat212/template/internal/domain/trip"
	"github.com/google/uuid"
)

type usecase interface {
	Execute(ctx context.Context, id uuid.UUID) (domain.Trip, error)
}
