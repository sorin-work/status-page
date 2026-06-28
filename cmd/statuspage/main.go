// Command statuspage runs the uptime monitor and serves the status page.
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
	_ "time/tzdata" // TIMEZONE works in minimal containers too

	"github.com/sorin-work/status-page/internal/alerting"
	"github.com/sorin-work/status-page/internal/auth"
	"github.com/sorin-work/status-page/internal/config"
	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/notify"
	"github.com/sorin-work/status-page/internal/scheduler"
	"github.com/sorin-work/status-page/internal/seed"
	"github.com/sorin-work/status-page/internal/store"
	"github.com/sorin-work/status-page/internal/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		return err
	}

	if err := ensureAdmin(ctx, st, cfg, logger); err != nil {
		return err
	}

	if cfg.SeedDemo {
		if _, err := seed.Demo(ctx, st, seed.Options{TargetURL: cfg.DemoTargetURL, Logger: logger}); err != nil {
			return fmt.Errorf("seed demo data: %w", err)
		}
	}

	sched := scheduler.New(st, monitor.NewChecker(), cfg.CheckWorkers, logger.With("component", "scheduler"))
	sched.Jitter = 10 * time.Second
	dispatcher := notify.NewDispatcher(st, logger.With("component", "webhooks"))
	dispatcher.Start(2)
	defer dispatcher.Close()

	alerts := alerting.New(st, logger.With("component", "alerting"))
	alerts.Publisher = dispatcher
	sched.OnTransition = alerts.HandleTransition
	schedDone := make(chan struct{})
	go func() {
		sched.Run(ctx)
		close(schedDone)
	}()

	handler, err := web.New(web.Options{
		Store:        st,
		Logger:       logger.With("component", "web"),
		SiteTitle:    cfg.SiteTitle,
		SessionTTL:   cfg.SessionTTL,
		CookieSecure: cfg.CookieSecure,
		Location:     cfg.Location,
		APIToken:     cfg.APIToken,
		Notifier:     dispatcher,
	})
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	err = srv.Shutdown(shutdownCtx)
	stop()
	<-schedDone
	return err
}

// ensureAdmin creates the first admin account when there is none. Without
// ADMIN_PASSWORD a random password is generated and logged once.
func ensureAdmin(ctx context.Context, st *store.Store, cfg config.Config, logger *slog.Logger) error {
	n, err := st.CountUsers(ctx)
	if err != nil || n > 0 {
		return err
	}
	password := cfg.AdminPassword
	generated := password == ""
	if generated {
		token, err := auth.NewToken()
		if err != nil {
			return err
		}
		password = token[:16]
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if _, err := st.CreateUser(ctx, cfg.AdminUsername, hash); err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	if generated {
		logger.Warn("created admin account with a generated password; set ADMIN_PASSWORD to choose one",
			"username", cfg.AdminUsername, "password", password)
	} else {
		logger.Info("created admin account", "username", cfg.AdminUsername)
	}
	return nil
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l}))
}
