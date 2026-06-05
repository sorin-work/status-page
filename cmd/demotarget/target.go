package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// services that can be switched off with POST /control/outage.
var services = []string{"storefront", "api", "checkout", "search"}

type target struct {
	logger *slog.Logger

	mu   sync.Mutex
	down map[string]bool

	// failRate is the share of /checkout requests that fail.
	failRate float64
	sleep    func(time.Duration)
	rand     func() float64
}

func newTarget(logger *slog.Logger) *target {
	return &target{logger: logger, down: map[string]bool{}, failRate: 0.03, sleep: time.Sleep, rand: rand.Float64}
}

func (t *target) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /{$}", t.service("storefront", t.storefront))
	mux.Handle("GET /api/health", t.service("api", t.apiHealth))
	mux.Handle("GET /checkout", t.service("checkout", t.checkout))
	mux.Handle("GET /search", t.service("search", t.search))
	mux.HandleFunc("GET /control", t.controlState)
	mux.HandleFunc("POST /control/outage", t.controlOutage)
	mux.HandleFunc("GET /status/{code}", t.status)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	return mux
}

// service answers 503 while the named service is switched off.
func (t *target) service(name string, h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.mu.Lock()
		down := t.down[name]
		t.mu.Unlock()
		if down {
			http.Error(w, name+" is unavailable (simulated outage)", http.StatusServiceUnavailable)
			return
		}
		h(w, r)
	})
}

// controlOutage switches a service off or on:
//
//	curl -X POST 'localhost:8081/control/outage?service=api&state=on'
func (t *target) controlOutage(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("service")
	if !slices.Contains(services, name) {
		http.Error(w, "service must be one of "+strings.Join(services, ", "), http.StatusBadRequest)
		return
	}
	var down bool
	switch r.URL.Query().Get("state") {
	case "on":
		down = true
	case "off":
	default:
		http.Error(w, "state must be on or off", http.StatusBadRequest)
		return
	}
	t.mu.Lock()
	t.down[name] = down
	t.mu.Unlock()
	t.logger.Info("outage switched", "service", name, "down", down)
	t.controlState(w, r)
}

func (t *target) controlState(w http.ResponseWriter, _ *http.Request) {
	t.mu.Lock()
	state := map[string]string{}
	for _, s := range services {
		state[s] = "up"
		if t.down[s] {
			state[s] = "down"
		}
	}
	t.mu.Unlock()
	writeJSON(w, http.StatusOK, state)
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
