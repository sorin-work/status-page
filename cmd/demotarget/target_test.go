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

func TestOutageToggle(t *testing.T) {
	h := testTarget().routes()
	if rec := do(t, h, http.MethodPost, "/control/outage?service=api&state=on"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"api":"down"`) {
		t.Fatalf("switch off: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodGet, "/api/health"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("api during outage: %d", rec.Code)
	}
	if rec := do(t, h, http.MethodGet, "/"); rec.Code != http.StatusOK {
		t.Errorf("storefront should be unaffected: %d", rec.Code)
	}
	do(t, h, http.MethodPost, "/control/outage?service=api&state=off")
	if rec := do(t, h, http.MethodGet, "/api/health"); rec.Code != http.StatusOK {
		t.Errorf("api after outage: %d", rec.Code)
	}
	for _, q := range []string{"service=db&state=on", "service=api&state=maybe"} {
		if rec := do(t, h, http.MethodPost, "/control/outage?"+q); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", q, rec.Code)
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
