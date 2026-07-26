package web

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestsAreLogged(t *testing.T) {
	srv, _ := newTestServer(t)
	var buf bytes.Buffer
	srv.logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	get(t, srv, "/nope")
	get(t, srv, "/healthz")
	out := buf.String()
	if !strings.Contains(out, "path=/nope") || !strings.Contains(out, "status=404") {
		t.Errorf("request not logged: %s", out)
	}
	if strings.Contains(out, "path=/healthz") {
		t.Error("health checks should only be logged at debug level")
	}
}

func TestRequestID(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := get(t, srv, "/healthz")
	if id := rec.Header().Get(requestIDHeader); len(id) != 12 {
		t.Errorf("generated id = %q", id)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set(requestIDHeader, "edge-4f2a")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if got := rec.Header().Get(requestIDHeader); got != "edge-4f2a" {
		t.Errorf("incoming id not kept: %q", got)
	}

	req.Header.Set(requestIDHeader, "bad id\nwith newline")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if got := rec.Header().Get(requestIDHeader); strings.Contains(got, " ") || len(got) != 12 {
		t.Errorf("unsafe id accepted: %q", got)
	}
}
