package seed

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "seed.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return st
}

func opts() Options {
	return Options{TargetURL: "http://demo-target:8081", Now: time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC), Logger: slog.New(slog.DiscardHandler)}
}

func TestDemoSeedsOnce(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	done, err := Demo(ctx, st, opts())
	if err != nil || !done {
		t.Fatalf("first seed: %v %v", done, err)
	}
	monitors, _ := st.ListMonitors(ctx)
	components, _ := st.ListComponents(ctx)
	if len(monitors) != 6 || len(components) != 5 {
		t.Fatalf("%d monitors, %d components", len(monitors), len(components))
	}
	var tcp string
	for _, m := range monitors {
		if m.Type == "tcp" {
			tcp = m.Target
		}
	}
	if tcp != "demo-target:8081" {
		t.Errorf("tcp target = %q", tcp)
	}

	done, err = Demo(ctx, st, opts())
	if err != nil || done {
		t.Fatalf("second seed should be skipped: %v %v", done, err)
	}
}

func TestDemoRejectsBadURL(t *testing.T) {
	o := opts()
	o.TargetURL = "not a url"
	if _, err := Demo(context.Background(), newStore(t), o); err == nil {
		t.Fatal("expected an error")
	}
}
