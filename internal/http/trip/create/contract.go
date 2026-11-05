package create

import (
	"context"

	domain "github.com/MrCat212/template/internal/domain/trip"
	usecasecreate "github.com/MrCat212/template/internal/usecase/trip/create"
)

type usecase interface {
	Execute(ctx context.Context, input usecasecreate.Input) (domain.Trip, error)
}
