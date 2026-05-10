package web

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/incident"
	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/store"
)

func TestStatusPageBars(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	comp, _ := st.CreateComponent(ctx, store.Component{Name: "API"})
	m := addMonitor(t, st, "api")
	m.ComponentID = comp.ID
	if _, err := st.UpdateMonitor(ctx, m); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordCheck(ctx, monitor.Result{MonitorID: m.ID, CheckedAt: time.Now(), Up: true}); err != nil {
		t.Fatal(err)
	}
	body := get(t, srv, "/").Body.String()
	if n := strings.Count(body, `class="bar `); n != 90 {
		t.Errorf("%d bars, want 90", n)
	}
	if n := strings.Count(body, `class="bar up"`); n != 1 {
		t.Errorf("%d green bars, want 1", n)
	}
	if !strings.Contains(body, "100.00% uptime") {
		t.Error("missing 90 day uptime")
	}
}

func TestStatusPageActiveIncident(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	comp, _ := st.CreateComponent(ctx, store.Component{Name: "Checkout"})
	_, err := st.CreateIncident(ctx, incident.Incident{
		Title: "Card payments failing", Status: incident.StatusIdentified, Impact: incident.ImpactMajor, ComponentID: comp.ID,
	}, "The card processor is returning errors.")
	if err != nil {
		t.Fatal(err)
	}
	body := get(t, srv, "/").Body.String()
	for _, want := range []string{"Card payments failing", "The card processor is returning errors.", "Major outage"} {
		if !strings.Contains(body, want) {
			t.Errorf("status page is missing %q", want)
		}
	}
}

func TestStatusPagePastIncidents(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	if body := get(t, srv, "/").Body.String(); !strings.Contains(body, "No incidents reported") {
		t.Error("missing empty state for past incidents")
	}
	for _, title := range []string{"DNS outage", "Slow search"} {
		inc, err := st.CreateIncident(ctx, incident.Incident{Title: title, Status: incident.StatusInvestigating, Impact: incident.ImpactMinor, StartedAt: time.Now().Add(-2 * time.Hour)}, "Looking.")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddIncidentUpdate(ctx, inc.ID, incident.StatusResolved, "All good now.", time.Now().Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	body := get(t, srv, "/").Body.String()
	for _, want := range []string{"DNS outage", "Slow search", "All good now.", "lasted 1h"} {
		if !strings.Contains(body, want) {
			t.Errorf("status page is missing %q", want)
		}
	}
}

func TestStatusPageEmpty(t *testing.T) {
	srv, _ := newTestServer(t)
	body := get(t, srv, "/").Body.String()
	if !strings.Contains(body, "Nothing to report yet") || !strings.Contains(body, "Status unknown") {
		t.Error("unexpected empty status page")
	}
}

func TestStatusPageComponents(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	api, _ := st.CreateComponent(ctx, store.Component{Name: "API", Description: "REST and webhooks"})
	web, _ := st.CreateComponent(ctx, store.Component{Name: "Website"})
	for _, tc := range []struct {
		comp   int64
		status monitor.Status
	}{{api.ID, monitor.StatusUp}, {api.ID, monitor.StatusDown}, {web.ID, monitor.StatusUp}} {
		m := addMonitor(t, st, "m")
		m.ComponentID = tc.comp
		if _, err := st.UpdateMonitor(ctx, m); err != nil {
			t.Fatal(err)
		}
		if err := st.SetMonitorStatus(ctx, m.ID, tc.status); err != nil {
			t.Fatal(err)
		}
	}
	body := get(t, srv, "/").Body.String()
	for _, want := range []string{"Some systems are experiencing issues", "REST and webhooks", "Partial outage", "Operational"} {
		if !strings.Contains(body, want) {
			t.Errorf("status page is missing %q", want)
		}
	}
}
