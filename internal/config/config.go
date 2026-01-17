// Package config reads the application settings from environment variables.
package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Config holds every setting the application needs at startup.
type Config struct {
	// Addr is the address the HTTP server listens on.
	Addr string
	// DatabasePath is the SQLite database file.
	DatabasePath string
	// LogLevel is one of debug, info, warn or error.
	LogLevel string
	// ShutdownTimeout bounds how long a graceful shutdown may take.
	ShutdownTimeout time.Duration
	// CheckWorkers is how many checks may run at the same time.
	CheckWorkers int
}

// Load builds a Config from the given lookup function, usually os.Getenv.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Addr:            ":4417",
		DatabasePath:    "data/status.db",
		LogLevel:        "info",
		ShutdownTimeout: 10 * time.Second,
		CheckWorkers:    8,
	}

	if v := strings.TrimSpace(getenv("LISTEN_ADDR")); v != "" {
		cfg.Addr = v
	}
	if v := strings.TrimSpace(getenv("DATABASE_PATH")); v != "" {
		cfg.DatabasePath = v
	}
	if v := strings.TrimSpace(getenv("LOG_LEVEL")); v != "" {
		v = strings.ToLower(v)
		switch v {
		case "debug", "info", "warn", "error":
			cfg.LogLevel = v
		default:
			return Config{}, fmt.Errorf("LOG_LEVEL: unknown level %q", v)
		}
	}
	if v := strings.TrimSpace(getenv("SHUTDOWN_TIMEOUT")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT: invalid duration %q", v)
		}
		cfg.ShutdownTimeout = d
	}
	if v := strings.TrimSpace(getenv("CHECK_WORKERS")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 256 {
			return Config{}, fmt.Errorf("CHECK_WORKERS: expected a number between 1 and 256, got %q", v)
		}
		cfg.CheckWorkers = n
	}
	return cfg, nil
}
