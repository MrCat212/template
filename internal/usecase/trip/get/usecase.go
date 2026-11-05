package get

import (
	"context"

	"github.com/MrCat212/template/internal/domain/trip"
	"github.com/google/uuid"
)

type Usecase struct {
	repository tripRepository
}

func New(repository tripRepository) *Usecase {
	return &Usecase{
		repository: repository,
	}
}

func (u *Usecase) Execute(ctx context.Context, id uuid.UUID) (trip.Trip, error) {
	return u.repository.GetByID(ctx, id)
}
