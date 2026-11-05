package container

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MrCat212/template/internal/config"
	api "github.com/MrCat212/template/internal/generated"
	httpserver "github.com/MrCat212/template/internal/http"
	"github.com/MrCat212/template/internal/http/domainerror"
	"github.com/MrCat212/template/internal/http/middleware"
	createhandler "github.com/MrCat212/template/internal/http/trip/create"
	finishhandler "github.com/MrCat212/template/internal/http/trip/finish"
	gethandler "github.com/MrCat212/template/internal/http/trip/get"
	"github.com/MrCat212/template/internal/infra/postgres"
	"github.com/MrCat212/template/internal/infra/telemetry"
	triprepo "github.com/MrCat212/template/internal/repository/trip"
	createuc "github.com/MrCat212/template/internal/usecase/trip/create"
	finishuc "github.com/MrCat212/template/internal/usecase/trip/finish"
	getuc "github.com/MrCat212/template/internal/usecase/trip/get"
)

type App struct {
	Server    *http.Server
	Logger    *slog.Logger
	pool      *pgxpool.Pool
	telemetry *telemetry.Telemetry
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	signals, err := telemetry.New(ctx, cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("initialize telemetry: %w", err)
	}

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		_ = signals.Shutdown(ctx)
		return nil, fmt.Errorf("initialize postgres: %w", err)
	}

	repo := triprepo.NewRepository(pool, cfg.DatabaseQueryTimeout)
	tx := postgres.NewTxManager(pool, cfg.DatabaseQueryTimeout)

	create := createhandler.New(createuc.New(repo, tx), signals)
	get := gethandler.New(getuc.New(repo))
	finish := finishhandler.New(finishuc.New(repo, tx), signals)

	apiServer := httpserver.New(create, get, finish, pool, cfg.DatabaseQueryTimeout)
	router := chi.NewRouter()
	router.Use(middleware.Recover(signals.Logger))
	router.Use(middleware.Tracing())
	router.Use(middleware.Logging(signals.Logger))
	router.Use(middleware.Metrics(signals))

	handler := api.HandlerWithOptions(apiServer, api.ChiServerOptions{
		BaseRouter:       router,
		ErrorHandlerFunc: domainerror.HandleAPIError,
	})

	return &App{
		Logger:    signals.Logger,
		pool:      pool,
		telemetry: signals,
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

func (a *App) Close(ctx context.Context) error {
	a.pool.Close()
	return a.telemetry.Shutdown(ctx)
}
