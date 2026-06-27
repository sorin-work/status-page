package alerting

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/incident"
	"github.com/sorin-work/status-page/internal/maintenance"
	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/notify"
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
	comp, err := st.CreateComponent(ctx, store.Component{Name: "Shop"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := st.CreateMonitor(ctx, monitor.Monitor{Name: "Checkout", Type: monitor.TypeHTTP, Target: "https://x.test", Interval: time.Minute, Timeout: time.Second, ComponentID: comp.ID})
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
	if inc.Title != "Checkout is down" || inc.Impact != incident.ImpactMajor || !inc.StartedAt.Equal(at) || inc.ComponentID != m.ComponentID {
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

func TestRecoveryResolvesIncident(t *testing.T) {
	mgr, st, m := setup(t)
	ctx := context.Background()
	down := time.Date(2026, 4, 12, 14, 0, 0, 0, time.UTC)
	mgr.HandleTransition(ctx, transition(m, monitor.StatusUp, monitor.StatusDown, down, "timeout"))
	mgr.HandleTransition(ctx, transition(m, monitor.StatusDown, monitor.StatusUp, down.Add(65*time.Minute), ""))

	list, _ := st.ListIncidents(ctx, 10, 0)
	if len(list) != 1 {
		t.Fatalf("%d incidents", len(list))
	}
	inc, _ := st.GetIncident(ctx, list[0].ID)
	if inc.Status != incident.StatusResolved || !inc.ResolvedAt.Equal(down.Add(65*time.Minute)) {
		t.Errorf("incident = %+v", inc)
	}
	if got := inc.Updates[0].Message; got != "Checkout is responding again. Total downtime: 1h 5m." {
		t.Errorf("message = %q", got)
	}
	if _, err := st.OpenIncidentForMonitor(ctx, m.ID); err == nil {
		t.Error("incident still open")
	}
}

func TestRecoveryWithoutIncident(t *testing.T) {
	mgr, st, m := setup(t)
	ctx := context.Background()
	mgr.HandleTransition(ctx, transition(m, monitor.StatusDown, monitor.StatusUp, time.Now(), ""))
	if n, _ := st.CountIncidents(ctx); n != 0 {
		t.Errorf("%d incidents", n)
	}
}

func TestHumanDuration(t *testing.T) {
	cases := map[time.Duration]string{
		20 * time.Second:  "less than a minute",
		50 * time.Second:  "1m",
		45 * time.Minute:  "45m",
		2 * time.Hour:     "2h",
		150 * time.Minute: "2h 30m",
	}
	for d, want := range cases {
		if got := humanDuration(d); got != want {
			t.Errorf("%v: got %q, want %q", d, got, want)
		}
	}
}

func TestNoIncidentDuringMaintenance(t *testing.T) {
	mgr, st, m := setup(t)
	ctx := context.Background()
	now := time.Now()
	if _, err := st.CreateMaintenance(ctx, maintenance.Window{Title: "upgrade", ComponentID: m.ComponentID, StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	mgr.HandleTransition(ctx, transition(m, monitor.StatusUp, monitor.StatusDown, now, "refused"))
	if n, _ := st.CountIncidents(ctx); n != 0 {
		t.Errorf("%d incidents during maintenance", n)
	}
}

func TestMaintenanceOfOtherComponent(t *testing.T) {
	mgr, st, m := setup(t)
	ctx := context.Background()
	other, _ := st.CreateComponent(ctx, store.Component{Name: "Other"})
	now := time.Now()
	if _, err := st.CreateMaintenance(ctx, maintenance.Window{Title: "upgrade", ComponentID: other.ID, StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	// A window that has not started yet does not count either.
	if _, err := st.CreateMaintenance(ctx, maintenance.Window{Title: "later", StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	mgr.HandleTransition(ctx, transition(m, monitor.StatusUp, monitor.StatusDown, now, "refused"))
	if n, _ := st.CountIncidents(ctx); n != 1 {
		t.Errorf("%d incidents, want 1", n)
	}
}

type recorder struct{ events []notify.Event }

func (r *recorder) Publish(_ context.Context, ev notify.Event) { r.events = append(r.events, ev) }

func (r *recorder) types() []string {
	var out []string
	for _, ev := range r.events {
		out = append(out, ev.Type)
	}
	return out
}

func TestPublishesEvents(t *testing.T) {
	mgr, _, m := setup(t)
	rec := &recorder{}
	mgr.Publisher = rec
	ctx := context.Background()
	now := time.Now()
	mgr.HandleTransition(ctx, transition(m, monitor.StatusUnknown, monitor.StatusUp, now, ""))
	mgr.HandleTransition(ctx, transition(m, monitor.StatusUp, monitor.StatusDown, now, "refused"))
	mgr.HandleTransition(ctx, transition(m, monitor.StatusDown, monitor.StatusUp, now.Add(time.Minute), ""))

	want := []string{"monitor.down", "incident.created", "monitor.up", "incident.updated"}
	got := rec.types()
	if len(got) != len(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("events = %v, want %v", got, want)
		}
	}
	if rec.events[0].Monitor.Error != "refused" || rec.events[3].Incident.Status != "resolved" {
		t.Errorf("payloads: %+v %+v", rec.events[0].Monitor, rec.events[3].Incident)
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
