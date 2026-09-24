package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr        string
	LogLevel        string
	ShutdownTimeout time.Duration

	DatabaseURL             string
	DatabaseMaxConns        int32
	DatabaseMinConns        int32
	DatabaseMaxConnLifetime time.Duration
	DatabaseConnectTimeout  time.Duration
	DatabaseQueryTimeout    time.Duration
}

func Load() (Config, error) {
	var cfg Config

	cfg.HTTPAddr = os.Getenv("HTTP_ADDR")
	cfg.LogLevel = os.Getenv("LOG_LEVEL")
	cfg.DatabaseURL = os.Getenv("DATABASE_URL")

	if cfg.HTTPAddr == "" {
		return Config{}, fmt.Errorf("HTTP_ADDR is required")
	}

	if cfg.LogLevel == "" {
		return Config{}, fmt.Errorf("LOG_LEVEL is required")
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	var err error

	cfg.ShutdownTimeout, err = time.ParseDuration(os.Getenv("SHUTDOWN_TIMEOUT"))
	if err != nil {
		return Config{}, fmt.Errorf("parse SHUTDOWN_TIMEOUT: %w", err)
	}

	maxConns, err := strconv.ParseInt(os.Getenv("DATABASE_MAX_CONNS"), 10, 32)
	if err != nil {
		return Config{}, fmt.Errorf("parse DATABASE_MAX_CONNS: %w", err)
	}
	cfg.DatabaseMaxConns = int32(maxConns)

	minConns, err := strconv.ParseInt(os.Getenv("DATABASE_MIN_CONNS"), 10, 32)
	if err != nil {
		return Config{}, fmt.Errorf("parse DATABASE_MIN_CONNS: %w", err)
	}
	cfg.DatabaseMinConns = int32(minConns)

	cfg.DatabaseMaxConnLifetime, err = time.ParseDuration(os.Getenv("DATABASE_MAX_CONN_LIFETIME"))
	if err != nil {
		return Config{}, fmt.Errorf("parse DATABASE_MAX_CONN_LIFETIME: %w", err)
	}

	cfg.DatabaseConnectTimeout, err = time.ParseDuration(os.Getenv("DATABASE_CONNECT_TIMEOUT"))
	if err != nil {
		return Config{}, fmt.Errorf("parse DATABASE_CONNECT_TIMEOUT: %w", err)
	}

	cfg.DatabaseQueryTimeout, err = time.ParseDuration(os.Getenv("DATABASE_QUERY_TIMEOUT"))
	if err != nil {
		return Config{}, fmt.Errorf("parse DATABASE_QUERY_TIMEOUT: %w", err)
	}

	return cfg, nil
}
