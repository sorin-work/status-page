package e2e

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/seed"
	"github.com/sorin-work/status-page/internal/store"
	"github.com/sorin-work/status-page/internal/web"
)

// The demo data must render: it is the first thing people see.
func TestDemoDataRenders(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "demo.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.Demo(ctx, st, seed.Options{TargetURL: "http://demo-target:8081", Now: time.Now(), Logger: slog.New(slog.DiscardHandler)}); err != nil {
		t.Fatal(err)
	}
	srv, err := web.New(web.Options{Store: st, SiteTitle: "Acme Status", Logger: slog.New(slog.DiscardHandler), APIToken: "demo-api-token-change-me"})
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status page: %d", rec.Code)
	}
	for _, want := range []string{"Website", "Checkout", "Database", "Database maintenance", "Checkout is down", "Elevated API error rates"} {
		if !strings.Contains(body, want) {
			t.Errorf("demo status page is missing %q", want)
		}
	}
	if n := strings.Count(body, `class="bar down"`) + strings.Count(body, `class="bar degraded"`); n == 0 {
		t.Error("the demo history shows no bad days")
	}

	for _, path := range []string{"/history", "/api/status"} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: %d", path, rec.Code)
		}
	}
}
