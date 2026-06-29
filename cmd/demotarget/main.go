// Command demotarget is a small fake shop for the demo monitors to check.
// Some endpoints are slow or fail now and then, so the status page has
// something to show.
package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"
)

func main() {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	t := newTarget(logger)
	t.webhookSecret = os.Getenv("WEBHOOK_SECRET")
	srv := &http.Server{
		Addr:              addr,
		Handler:           t.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	logger.Info("demo target listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "err", err)
		os.Exit(1)
	}
}
