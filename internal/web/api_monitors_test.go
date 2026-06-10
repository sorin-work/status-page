package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/sorin-work/status-page/internal/store"
)

const testToken = "test-token-0123456789"

func newAPIServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	srv, err := New(Options{Store: st, APIToken: testToken, Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	return srv, st
}

// apiCall sends a request with the test token. body may be nil.
func apiCall(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return v
}

func TestAPIRequiresToken(t *testing.T) {
	srv, _ := newAPIServer(t)
	for _, auth := range []string{"", "Bearer wrong", "Basic abc", testToken} {
		req := httptest.NewRequest(http.MethodGet, "/api/monitors", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%q: got %d", auth, rec.Code)
		}
	}
}

func TestAPIDisabledWithoutToken(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/monitors", nil)
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got %d", rec.Code)
	}
}

func TestAPIListAndGetMonitors(t *testing.T) {
	srv, st := newAPIServer(t)
	m := addMonitor(t, st, "website")

	rec := apiCall(t, srv, http.MethodGet, "/api/monitors", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	list := decode[struct{ Monitors []apiMonitor }](t, rec)
	if len(list.Monitors) != 1 || list.Monitors[0].Name != "website" || list.Monitors[0].IntervalSeconds != 60 {
		t.Errorf("list = %+v", list)
	}

	rec = apiCall(t, srv, http.MethodGet, "/api/monitors/"+itoa(m.ID), nil)
	got := decode[apiMonitor](t, rec)
	if rec.Code != http.StatusOK || got.ID != m.ID || got.ComponentID != nil {
		t.Errorf("get: %d %+v", rec.Code, got)
	}
	if rec := apiCall(t, srv, http.MethodGet, "/api/monitors/999", nil); rec.Code != http.StatusNotFound {
		t.Errorf("missing: %d", rec.Code)
	}
}
