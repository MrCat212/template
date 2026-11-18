package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MrCat212/template/internal/config"
	"github.com/MrCat212/template/internal/gateway/push"
	api "github.com/MrCat212/template/internal/generated"
	httpserver "github.com/MrCat212/template/internal/http"
	"github.com/MrCat212/template/internal/http/domainerror"
	"github.com/MrCat212/template/internal/http/middleware"
	createhandler "github.com/MrCat212/template/internal/http/trip/create"
	finishhandler "github.com/MrCat212/template/internal/http/trip/finish"
	gethandler "github.com/MrCat212/template/internal/http/trip/get"
	positioncreatehandler "github.com/MrCat212/template/internal/http/trip/positions/create"
	positionlisthandler "github.com/MrCat212/template/internal/http/trip/positions/list"
	"github.com/MrCat212/template/internal/infra/postgres"
	"github.com/MrCat212/template/internal/infra/telemetry"
	triprepo "github.com/MrCat212/template/internal/repository/trip"
	createuc "github.com/MrCat212/template/internal/usecase/trip/create"
	finishuc "github.com/MrCat212/template/internal/usecase/trip/finish"
	getuc "github.com/MrCat212/template/internal/usecase/trip/get"
	positioncreateuc "github.com/MrCat212/template/internal/usecase/trip/positions/create"
	positionlistuc "github.com/MrCat212/template/internal/usecase/trip/positions/list"
	workerpositions "github.com/MrCat212/template/internal/worker/positions"
)

type App struct {
	Server          *http.Server
	Logger          *slog.Logger
	pool            *pgxpool.Pool
	telemetry       *telemetry.Telemetry
	positionWorkers *workerpositions.Pool
	pushClient      *push.Client
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

	pushClient, err := push.New(cfg.PushHTTPURL, cfg.PushRequestTimeout)
	if err != nil {
		pool.Close()
		_ = signals.Shutdown(ctx)
		return nil, fmt.Errorf("initialize push gateway: %w", err)
	}

	positionWorkers, err := workerpositions.New(
		repo,
		pushClient,
		signals.Logger,
		workerpositions.Config{
			CheckInterval:  cfg.PositionCheckInterval,
			StaleAfter:     cfg.PositionStaleAfter,
			Workers:        cfg.PositionWorkers,
			QueueSize:      cfg.PositionQueueSize,
			ScanBatchSize:  cfg.PositionScanBatchSize,
			RequestTimeout: cfg.PushRequestTimeout,
		},
		nil, // real time.Ticker; tests can inject a manual ticker
	)
	if err != nil {
		pushClient.Close()
		pool.Close()
		_ = signals.Shutdown(ctx)
		return nil, fmt.Errorf("initialize position workers: %w", err)
	}

	create := createhandler.New(createuc.New(repo, tx), signals)
	get := gethandler.New(getuc.New(repo))
	finish := finishhandler.New(finishuc.New(repo, tx), signals)
	positionCreate := positioncreatehandler.New(positioncreateuc.New(repo, tx, positionWorkers).WithMetrics(signals))
	positionList := positionlisthandler.New(positionlistuc.New(repo))

	apiServer := httpserver.New(create, get, finish, positionCreate, positionList, pool, cfg.DatabaseQueryTimeout)
	router := chi.NewRouter()
	router.Use(middleware.Recover(signals.Logger))
	router.Use(middleware.Tracing())
	router.Use(middleware.Logging(signals.Logger))
	router.Use(middleware.Metrics(signals))

	handler := api.HandlerWithOptions(apiServer, api.ChiServerOptions{
		BaseRouter:       router,
		ErrorHandlerFunc: domainerror.HandleAPIError,
	})

	positionWorkers.Start()

	return &App{
		Logger:          signals.Logger,
		pool:            pool,
		telemetry:       signals,
		positionWorkers: positionWorkers,
		pushClient:      pushClient,
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
	// HTTP Server.Shutdown is called first by main.run().
	// Then stop the scanner and drain/cancel the worker queue.
	stopErr := a.positionWorkers.Stop(ctx)
	a.pushClient.Close()
	a.pool.Close()
	return errors.Join(stopErr, a.telemetry.Shutdown(ctx))
}
