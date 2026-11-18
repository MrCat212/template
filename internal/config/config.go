package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr              string
	LogLevel              string
	ShutdownTimeout       time.Duration
	HTTPReadTimeout       time.Duration
	HTTPReadHeaderTimeout time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration

	DatabaseURL             string
	DatabaseMaxConns        int32
	DatabaseMinConns        int32
	DatabaseMaxConnLifetime time.Duration
	DatabaseConnectTimeout  time.Duration
	DatabaseQueryTimeout    time.Duration

	PositionCheckInterval time.Duration
	PositionStaleAfter    time.Duration
	PositionWorkers       int
	PositionQueueSize     int
	PositionScanBatchSize int
	PushHTTPURL           string
	PushRequestTimeout    time.Duration
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
	cfg.HTTPReadTimeout, err = time.ParseDuration(getEnv("HTTP_READ_TIMEOUT", "10s"))
	if err != nil {
		return Config{}, fmt.Errorf("parse HTTP_READ_TIMEOUT: %w", err)
	}

	cfg.HTTPReadHeaderTimeout, err = time.ParseDuration(getEnv("HTTP_READ_HEADER_TIMEOUT", "5s"))
	if err != nil {
		return Config{}, fmt.Errorf("parse HTTP_READ_HEADER_TIMEOUT: %w", err)
	}

	cfg.HTTPWriteTimeout, err = time.ParseDuration(getEnv("HTTP_WRITE_TIMEOUT", "15s"))
	if err != nil {
		return Config{}, fmt.Errorf("parse HTTP_WRITE_TIMEOUT: %w", err)
	}

	cfg.HTTPIdleTimeout, err = time.ParseDuration(getEnv("HTTP_IDLE_TIMEOUT", "60s"))
	if err != nil {
		return Config{}, fmt.Errorf("parse HTTP_IDLE_TIMEOUT: %w", err)
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

	cfg.PositionCheckInterval, err = time.ParseDuration(getEnv("POSITION_CHECK_INTERVAL", "30s"))
	if err != nil || cfg.PositionCheckInterval <= 0 {
		return Config{}, fmt.Errorf("POSITION_CHECK_INTERVAL must be a positive duration")
	}
	cfg.PositionStaleAfter, err = time.ParseDuration(getEnv("POSITION_STALE_AFTER", "60s"))
	if err != nil || cfg.PositionStaleAfter <= 0 {
		return Config{}, fmt.Errorf("POSITION_STALE_AFTER must be a positive duration")
	}
	cfg.PositionWorkers, err = positiveInt("POSITION_WORKERS", "4")
	if err != nil {
		return Config{}, err
	}
	cfg.PositionQueueSize, err = positiveInt("POSITION_QUEUE_SIZE", "100")
	if err != nil {
		return Config{}, err
	}
	cfg.PositionScanBatchSize, err = positiveInt("POSITION_SCAN_BATCH_SIZE", "100")
	if err != nil {
		return Config{}, err
	}
	cfg.PushRequestTimeout, err = time.ParseDuration(getEnv("PUSH_REQUEST_TIMEOUT", "1s"))
	if err != nil || cfg.PushRequestTimeout <= 0 {
		return Config{}, fmt.Errorf("PUSH_REQUEST_TIMEOUT must be a positive duration")
	}
	cfg.PushHTTPURL = os.Getenv("PUSH_HTTP_URL")
	if cfg.PushHTTPURL == "" {
		return Config{}, fmt.Errorf("PUSH_HTTP_URL is required")
	}

	return cfg, nil
}

func positiveInt(name, fallback string) (int, error) {
	n, err := strconv.Atoi(getEnv(name, fallback))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return n, nil
}

func getEnv(name, defaultValue string) string {
	value := os.Getenv(name)

	if value == "" {
		return defaultValue
	}

	return value
}
