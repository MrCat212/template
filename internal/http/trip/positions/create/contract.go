//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package create

import (
	"context"

	domain "github.com/MrCat212/template/internal/domain/trip"
	positioncreate "github.com/MrCat212/template/internal/usecase/trip/positions/create"
)

type usecase interface {
	Execute(ctx context.Context, input positioncreate.Input) (domain.Position, error)
}
