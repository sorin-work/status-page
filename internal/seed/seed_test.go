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

func TestDemoHistory(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	o := opts()
	if _, err := Demo(ctx, st, o); err != nil {
		t.Fatal(err)
	}
	monitors, _ := st.ListMonitors(ctx)
	byName := map[string]int64{}
	for _, m := range monitors {
		byName[m.Name] = m.ID
	}
	checkout := byName["Checkout"]
	stats, err := st.DailyStats(ctx, checkout, o.Now.AddDate(0, 0, -89), o.Now)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 90 {
		t.Errorf("checkout has %d days of history, want 90", len(stats))
	}
	up, total, err := st.CountChecksSince(ctx, checkout, o.Now.Add(-7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if total != 7*24*6 || total-up != 4 {
		t.Errorf("checkout raw checks: %d up of %d", up, total)
	}
	// The internal monitor has no profile and no history.
	if n, _ := st.CountChecks(ctx, byName["Status endpoint (internal)"]); n != 0 {
		t.Errorf("internal monitor has %d checks", n)
	}
}

func TestDemoIncidentsAndMaintenance(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	o := opts()
	if _, err := Demo(ctx, st, o); err != nil {
		t.Fatal(err)
	}
	list, err := st.ListIncidents(ctx, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != len(demoIncidents) {
		t.Fatalf("%d incidents", len(list))
	}
	for _, inc := range list {
		if !inc.Resolved() || inc.ComponentID == 0 {
			t.Errorf("incident %q: resolved=%v component=%d", inc.Title, inc.Resolved(), inc.ComponentID)
		}
	}
	if open, _ := st.ListOpenIncidents(ctx); len(open) != 0 {
		t.Errorf("%d open incidents", len(open))
	}
	current, _ := st.ListCurrentMaintenance(ctx, o.Now)
	if len(current) != 1 || !current[0].Upcoming(o.Now) {
		t.Errorf("upcoming maintenance = %+v", current)
	}
}

func TestDemoRejectsBadURL(t *testing.T) {
	o := opts()
	o.TargetURL = "not a url"
	if _, err := Demo(context.Background(), newStore(t), o); err == nil {
		t.Fatal("expected an error")
	}
}
