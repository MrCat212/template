package get

import (
	"context"

	domain "github.com/MrCat212/template/internal/domain/trip"
	"github.com/google/uuid"
)

type usecase interface {
	Execute(ctx context.Context, id uuid.UUID) (domain.Trip, error)
}
