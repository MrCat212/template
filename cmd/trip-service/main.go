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
	"github.com/MrCat212/template/internal/container"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	app, err := container.New(context.Background(), cfg)
	if err != nil {
		log.Fatalf("start application: %v", err)
	}
	defer app.Close()

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("HTTP server started on %s", cfg.HTTPAddr)
		serverErr <- app.Server.ListenAndServe()
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := app.Server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
		_ = app.Server.Close()
	}
	log.Println("HTTP server stopped")
}
