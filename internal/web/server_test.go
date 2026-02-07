package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sorin-work/status-page/internal/store"
)

func newTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "web.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	srv, err := New(Options{Store: st, SiteTitle: "Acme Status"})
	if err != nil {
		t.Fatal(err)
	}
	return srv, st
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHealth(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := get(t, srv, "/healthz")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "ok" {
		t.Fatalf("healthz = %d %q", rec.Code, rec.Body.String())
	}
}

func TestStaticFiles(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := get(t, srv, "/static/style.css")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("style.css = %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
}

func TestUnknownPath(t *testing.T) {
	srv, _ := newTestServer(t)
	if rec := get(t, srv, "/nope"); rec.Code != http.StatusNotFound {
		t.Fatalf("got %d", rec.Code)
	}
}
