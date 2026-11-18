package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/MrCat212/template/internal/domain/trip"
)

type Usecase struct {
	repository positionRepository
}

func New(repository positionRepository) *Usecase {
	return &Usecase{
		repository: repository,
	}
}

func (u *Usecase) Execute(ctx context.Context, tripID uuid.UUID) ([]trip.Position, error) {
	return u.repository.ListPositions(ctx, tripID)
}
