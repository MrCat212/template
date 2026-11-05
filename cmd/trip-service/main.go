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
	"time"

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

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- app.Server.ListenAndServe()
	}()

	app.Logger.InfoContext(stopCtx, "server started", "addr", cfg.HTTPAddr)

	var listenErr error
	select {
	case <-stopCtx.Done():
		app.Logger.Info("server stopping")
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenErr = fmt.Errorf("HTTP server: %w", err)
		}
	}

	watchdog := time.AfterFunc(cfg.ShutdownTimeout, func() {
		app.Logger.Error("shutdown timeout exceeded; forcing process exit")
		os.Exit(1)
	})
	defer watchdog.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	var shutdownErr error
	if err := app.Server.Shutdown(shutdownCtx); err != nil {
		app.Logger.ErrorContext(shutdownCtx, "HTTP shutdown failed", "error", err)
		_ = app.Server.Close()
		shutdownErr = fmt.Errorf("HTTP shutdown: %w", err)
	}

	closeErr := app.Close(shutdownCtx)
	if closeErr != nil {
		app.Logger.ErrorContext(shutdownCtx, "resource shutdown failed", "error", closeErr)
	}
	app.Logger.Info("server stopped")

	return errors.Join(listenErr, shutdownErr, closeErr)
}
