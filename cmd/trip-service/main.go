package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/MrCat212/template/internal/config"
	"github.com/MrCat212/template/internal/container"
)

func main() {
	if err := run(); err != nil {
		slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "trip-service").Error(
			"service stopped with error", "error", err,
		)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	app, err := container.New(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("start application: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := app.Close(ctx); err != nil {
			app.Logger.ErrorContext(ctx, "telemetry shutdown failed", "error", err)
		}
	}()

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- app.Server.ListenAndServe()
	}()

	app.Logger.InfoContext(stopCtx, "server started", "addr", cfg.HTTPAddr)

	select {
	case <-stopCtx.Done():
		app.Logger.InfoContext(stopCtx, "server stopping")
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server: %w", err)
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := app.Server.Shutdown(shutdownCtx); err != nil {
		_ = app.Server.Close()
		return fmt.Errorf("HTTP shutdown: %w", err)
	}
	app.Logger.InfoContext(shutdownCtx, "server stopped")
	return nil
}
