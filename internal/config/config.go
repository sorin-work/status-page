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
	// LogFormat is text or json.
	LogFormat string
	// ShutdownTimeout bounds how long a graceful shutdown may take.
	ShutdownTimeout time.Duration
	// CheckWorkers is how many checks may run at the same time.
	CheckWorkers int
	// AdminUsername and AdminPassword create the first admin account
	// when the database has none.
	AdminUsername string
	AdminPassword string
	// SiteTitle is shown in the header of every page.
	SiteTitle string
	// SessionTTL is how long an admin stays logged in.
	SessionTTL time.Duration
	// CookieSecure marks the session cookie as HTTPS only.
	CookieSecure bool
	// Location is the time zone dates are shown in.
	Location *time.Location
	// SeedDemo fills an empty database with demo data.
	SeedDemo bool
	// DemoTargetURL is where the demo monitors point.
	DemoTargetURL string
	// APIToken enables the JSON API for clients that send it as a bearer
	// token.
	APIToken string
	// TrustProxy takes the client address from X-Forwarded-For, for
	// deployments behind a reverse proxy.
	TrustProxy bool
	// Retention is how long raw checks are kept. Daily rollups are kept
	// forever.
	Retention time.Duration
}

// Load builds a Config from the given lookup function, usually os.Getenv.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Addr:            ":4417",
		DatabasePath:    "data/status.db",
		LogLevel:        "info",
		LogFormat:       "text",
		ShutdownTimeout: 10 * time.Second,
		CheckWorkers:    8,
		AdminUsername:   "admin",
		SiteTitle:       "Status",
		SessionTTL:      7 * 24 * time.Hour,
		Location:        time.UTC,
		DemoTargetURL:   "http://localhost:8081",
		Retention:       30 * 24 * time.Hour,
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
	if v := strings.ToLower(strings.TrimSpace(getenv("LOG_FORMAT"))); v != "" {
		if v != "text" && v != "json" {
			return Config{}, fmt.Errorf("LOG_FORMAT: expected text or json, got %q", v)
		}
		cfg.LogFormat = v
	}
	if v := strings.TrimSpace(getenv("SHUTDOWN_TIMEOUT")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT: invalid duration %q", v)
		}
		cfg.ShutdownTimeout = d
	}
	if v := strings.TrimSpace(getenv("SITE_TITLE")); v != "" {
		cfg.SiteTitle = v
	}
	if v := strings.TrimSpace(getenv("ADMIN_USERNAME")); v != "" {
		cfg.AdminUsername = v
	}
	cfg.AdminPassword = getenv("ADMIN_PASSWORD")
	if cfg.AdminPassword != "" && len(cfg.AdminPassword) < 8 {
		return Config{}, fmt.Errorf("ADMIN_PASSWORD: must be at least 8 characters")
	}
	if v := strings.TrimSpace(getenv("SESSION_TTL")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < time.Minute {
			return Config{}, fmt.Errorf("SESSION_TTL: expected a duration of at least 1m, got %q", v)
		}
		cfg.SessionTTL = d
	}
	if v := getenv("COOKIE_SECURE"); v != "" {
		b, err := parseBool("COOKIE_SECURE", v)
		if err != nil {
			return Config{}, err
		}
		cfg.CookieSecure = b
	}
	if v := strings.TrimSpace(getenv("TIMEZONE")); v != "" {
		loc, err := time.LoadLocation(v)
		if err != nil {
			return Config{}, fmt.Errorf("TIMEZONE: unknown time zone %q", v)
		}
		cfg.Location = loc
	}
	if v := getenv("SEED_DEMO"); v != "" {
		b, err := parseBool("SEED_DEMO", v)
		if err != nil {
			return Config{}, err
		}
		cfg.SeedDemo = b
	}
	if v := strings.TrimSpace(getenv("DEMO_TARGET_URL")); v != "" {
		cfg.DemoTargetURL = v
	}
	cfg.APIToken = strings.TrimSpace(getenv("API_TOKEN"))
	if cfg.APIToken != "" && len(cfg.APIToken) < 16 {
		return Config{}, fmt.Errorf("API_TOKEN: must be at least 16 characters")
	}
	if v := getenv("TRUST_PROXY"); v != "" {
		b, err := parseBool("TRUST_PROXY", v)
		if err != nil {
			return Config{}, err
		}
		cfg.TrustProxy = b
	}
	if v := strings.TrimSpace(getenv("RETENTION_DAYS")); v != "" {
		n, err := strconv.Atoi(v)
		// The 7 day uptime is computed from raw checks, so keep at least
		// a little more than that.
		if err != nil || n < 8 || n > 3650 {
			return Config{}, fmt.Errorf("RETENTION_DAYS: expected a number of days between 8 and 3650, got %q", v)
		}
		cfg.Retention = time.Duration(n) * 24 * time.Hour
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

// parseBool accepts the usual spellings of true and false.
func parseBool(name, v string) (bool, error) {
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return false, fmt.Errorf("%s: expected true or false, got %q", name, v)
	}
	return b, nil
}
