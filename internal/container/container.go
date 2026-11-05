package container

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MrCat212/template/internal/config"
	api "github.com/MrCat212/template/internal/generated"
	httpserver "github.com/MrCat212/template/internal/http"
	"github.com/MrCat212/template/internal/http/domainerror"
	createhandler "github.com/MrCat212/template/internal/http/trip/create"
	finishhandler "github.com/MrCat212/template/internal/http/trip/finish"
	gethandler "github.com/MrCat212/template/internal/http/trip/get"
	"github.com/MrCat212/template/internal/infra/postgres"
	triprepo "github.com/MrCat212/template/internal/repository/trip"
	createuc "github.com/MrCat212/template/internal/usecase/trip/create"
	finishuc "github.com/MrCat212/template/internal/usecase/trip/finish"
	getuc "github.com/MrCat212/template/internal/usecase/trip/get"
)

type App struct {
	Server *http.Server
	pool   *pgxpool.Pool
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		return nil, err
	}

	repo := triprepo.NewRepository(pool, cfg.DatabaseQueryTimeout)
	tx := postgres.NewTxManager(pool)

	create := createhandler.New(createuc.New(repo, tx))
	get := gethandler.New(getuc.New(repo))
	finish := finishhandler.New(finishuc.New(repo, tx))

	apiServer := httpserver.New(create, get, finish, pool, cfg.DatabaseQueryTimeout)
	handler := api.HandlerWithOptions(apiServer, api.ChiServerOptions{
		ErrorHandlerFunc: domainerror.HandleAPIError,
	})

	return &App{
		pool: pool,
		Server: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           handler,
			ReadTimeout:       cfg.HTTPReadTimeout,
			ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
			WriteTimeout:      cfg.HTTPWriteTimeout,
			IdleTimeout:       cfg.HTTPIdleTimeout,
		},
	}, nil
}

func (a *App) Close() {
	a.pool.Close()
}
