package main

import (
	"context"
	"log"

	"github.com/MrCat212/template/internal/config"
	"github.com/MrCat212/template/internal/database"
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

	log.Println("database connection established")
}
