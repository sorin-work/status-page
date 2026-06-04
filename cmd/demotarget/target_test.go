package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testTarget() *target {
	t := newTarget(slog.New(slog.DiscardHandler))
	t.sleep = func(time.Duration) {}
	t.rand = func() float64 { return 0.5 }
	return t
}

func do(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestEndpoints(t *testing.T) {
	h := testTarget().routes()
	cases := []struct {
		path string
		code int
		body string
	}{
		{"/", http.StatusOK, "Acme Shop"},
		{"/api/health", http.StatusOK, `"status":"ok"`},
		{"/checkout", http.StatusOK, "ready"},
		{"/search?q=lamp", http.StatusOK, `"query":"lamp"`},
		{"/status/204", http.StatusNoContent, ""},
		{"/status/503", http.StatusServiceUnavailable, "status 503"},
		{"/status/42", http.StatusBadRequest, ""},
		{"/healthz", http.StatusOK, "ok"},
	}
	for _, tc := range cases {
		rec := do(t, h, http.MethodGet, tc.path)
		if rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.body) {
			t.Errorf("%s: %d %q", tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestCheckoutFailsSometimes(t *testing.T) {
	tg := testTarget()
	tg.rand = func() float64 { return 0.01 }
	if rec := do(t, tg.routes(), http.MethodGet, "/checkout"); rec.Code != http.StatusBadGateway {
		t.Errorf("got %d", rec.Code)
	}
}
