//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package create

import (
	"context"

	"github.com/google/uuid"

	domain "github.com/MrCat212/template/internal/domain/trip"
	usecasecreate "github.com/MrCat212/template/internal/usecase/trip/create"
)

type usecase interface {
	Execute(ctx context.Context, input usecasecreate.Input) (domain.Trip, error)
}

type eventRecorder interface {
	Created(ctx context.Context, id uuid.UUID)
}
