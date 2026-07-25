package web

import (
	"bytes"
	"log/slog"
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
