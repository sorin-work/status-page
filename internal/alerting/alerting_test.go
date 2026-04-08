package alerting

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/incident"
	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/scheduler"
	"github.com/sorin-work/status-page/internal/store"
)

func setup(t *testing.T) (*Manager, *store.Store, monitor.Monitor) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "alerting.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	m, err := st.CreateMonitor(ctx, monitor.Monitor{Name: "Checkout", Type: monitor.TypeHTTP, Target: "https://x.test", Interval: time.Minute, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return New(st, slog.New(slog.DiscardHandler)), st, m
}

func transition(m monitor.Monitor, from, to monitor.Status, at time.Time, errMsg string) scheduler.Transition {
	return scheduler.Transition{Monitor: m, From: from, To: to, Result: monitor.Result{MonitorID: m.ID, CheckedAt: at, Up: to == monitor.StatusUp, Error: errMsg}}
}

func TestDownOpensIncident(t *testing.T) {
	mgr, st, m := setup(t)
	ctx := context.Background()
	at := time.Date(2026, 4, 8, 22, 0, 0, 0, time.UTC)
	mgr.HandleTransition(ctx, transition(m, monitor.StatusUp, monitor.StatusDown, at, "connection refused"))

	inc, err := st.OpenIncidentForMonitor(ctx, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if inc.Title != "Checkout is down" || inc.Impact != incident.ImpactMajor || !inc.StartedAt.Equal(at) {
		t.Errorf("incident = %+v", inc)
	}
	full, _ := st.GetIncident(ctx, inc.ID)
	if full.Updates[0].Message != "Checks are failing: connection refused." {
		t.Errorf("message = %q", full.Updates[0].Message)
	}
}

func TestDownTwiceKeepsOneIncident(t *testing.T) {
	mgr, st, m := setup(t)
	ctx := context.Background()
	mgr.HandleTransition(ctx, transition(m, monitor.StatusUp, monitor.StatusDown, time.Now(), ""))
	// After a restart the scheduler sees the monitor go unknown -> down.
	mgr.HandleTransition(ctx, transition(m, monitor.StatusUnknown, monitor.StatusDown, time.Now(), ""))
	if n, _ := st.CountIncidents(ctx); n != 1 {
		t.Errorf("%d incidents, want 1", n)
	}
}

func TestFirstUpOpensNothing(t *testing.T) {
	mgr, st, m := setup(t)
	ctx := context.Background()
	mgr.HandleTransition(ctx, transition(m, monitor.StatusUnknown, monitor.StatusUp, time.Now(), ""))
	if n, _ := st.CountIncidents(ctx); n != 0 {
		t.Errorf("%d incidents, want 0", n)
	}
}
