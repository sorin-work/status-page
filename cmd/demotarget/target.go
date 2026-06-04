package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

type target struct {
	logger *slog.Logger
	// failRate is the share of /checkout requests that fail.
	failRate float64
	sleep    func(time.Duration)
	rand     func() float64
}

func newTarget(logger *slog.Logger) *target {
	return &target{logger: logger, failRate: 0.03, sleep: time.Sleep, rand: rand.Float64}
}

func (t *target) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", t.storefront)
	mux.HandleFunc("GET /api/health", t.apiHealth)
	mux.HandleFunc("GET /checkout", t.checkout)
	mux.HandleFunc("GET /search", t.search)
	mux.HandleFunc("GET /status/{code}", t.status)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	return mux
}

const storefrontHTML = `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Acme Shop</title></head>
<body>
  <h1>Acme Shop</h1>
  <p>Everything you never knew you needed. This is a demo service for status-page.</p>
</body>
</html>
`

func (t *target) storefront(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(storefrontHTML))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (t *target) apiHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": "2.14.1"})
}

func (t *target) checkout(w http.ResponseWriter, _ *http.Request) {
	if t.rand() < t.failRate {
		http.Error(w, "upstream payment gateway error", http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cart": "ready", "payment_providers": 3})
}

// search answers in 0.6 to 1.4 seconds, like a slow backend.
func (t *target) search(w http.ResponseWriter, r *http.Request) {
	t.sleep(time.Duration(600+t.rand()*800) * time.Millisecond)
	q := r.URL.Query().Get("q")
	writeJSON(w, http.StatusOK, map[string]any{"query": q, "results": []string{}})
}

func (t *target) status(w http.ResponseWriter, r *http.Request) {
	code, err := strconv.Atoi(r.PathValue("code"))
	if err != nil || code < 200 || code > 599 {
		http.Error(w, "status must be between 200 and 599", http.StatusBadRequest)
		return
	}
	w.WriteHeader(code)
	if code != http.StatusNoContent && code != http.StatusNotModified {
		_, _ = fmt.Fprintf(w, "status %d\n", code)
	}
}
