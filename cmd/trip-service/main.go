package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/MrCat212/template/internal/config"
	"github.com/MrCat212/template/internal/database"
	api "github.com/MrCat212/template/internal/generated"
	"github.com/MrCat212/template/internal/httpapi"
	"github.com/MrCat212/template/internal/trip"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	pool, err := database.NewPool(ctx, cfg)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	repository := trip.NewRepository(pool, cfg.DatabaseQueryTimeout)
	txManager := database.NewTxManager(pool)

	service := trip.NewService(repository, txManager)

	handler := httpapi.NewHandler(service, pool, cfg.DatabaseQueryTimeout)

	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: api.HandlerWithOptions(handler, api.ChiServerOptions{
			ErrorHandlerFunc: httpapi.HandleAPIError,
		}),
		ReadTimeout:       cfg.HTTPReadTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	stopCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	serverErr := make(chan error, 1)

	go func() {
		log.Printf("HTTP server started on %s", cfg.HTTPAddr)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case <-stopCtx.Done():
		log.Println("stopping HTTP server")

	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server error: %v", err)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		cfg.ShutdownTimeout,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
		_ = server.Close()
	}

	log.Println("HTTP server stopped")
}
