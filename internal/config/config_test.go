package config

import (
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":4417" {
		t.Errorf("Addr = %q", cfg.Addr)
	}
	if cfg.DatabasePath != "data/status.db" {
		t.Errorf("DatabasePath = %q", cfg.DatabasePath)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q", cfg.LogLevel)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %v", cfg.ShutdownTimeout)
	}
	if cfg.Location != time.UTC {
		t.Errorf("Location = %v", cfg.Location)
	}
	if cfg.AdminUsername != "admin" || cfg.AdminPassword != "" {
		t.Errorf("admin = %q / %q", cfg.AdminUsername, cfg.AdminPassword)
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"LISTEN_ADDR":      "127.0.0.1:9000",
		"DATABASE_PATH":    "/tmp/x.db",
		"LOG_LEVEL":        "DEBUG",
		"SHUTDOWN_TIMEOUT": "3s",
		"CHECK_WORKERS":    "3",
		"SESSION_TTL":      "12h",
		"COOKIE_SECURE":    "true",
		"TIMEZONE":         "Europe/Bucharest",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "127.0.0.1:9000" || cfg.DatabasePath != "/tmp/x.db" || cfg.LogLevel != "debug" || cfg.ShutdownTimeout != 3*time.Second || cfg.CheckWorkers != 3 ||
		cfg.SessionTTL != 12*time.Hour || !cfg.CookieSecure || cfg.Location.String() != "Europe/Bucharest" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestLoadInvalid(t *testing.T) {
	cases := map[string]map[string]string{
		"log level":        {"LOG_LEVEL": "verbose"},
		"shutdown timeout": {"SHUTDOWN_TIMEOUT": "soon"},
		"negative timeout": {"SHUTDOWN_TIMEOUT": "-1s"},
		"workers zero":     {"CHECK_WORKERS": "0"},
		"workers text":     {"CHECK_WORKERS": "many"},
		"short password":   {"ADMIN_PASSWORD": "short"},
		"session ttl":      {"SESSION_TTL": "5s"},
		"cookie secure":    {"COOKIE_SECURE": "maybe"},
		"timezone":         {"TIMEZONE": "Mars/Olympus"},
	}
	for name, m := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(env(m)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
