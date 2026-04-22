package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/incident"
	"github.com/sorin-work/status-page/internal/store"
)

func TestIncidentListEmpty(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, _ := login(t, srv)
	if body := getWith(t, srv, "/admin/incidents", c).Body.String(); !strings.Contains(body, "No incidents so far") {
		t.Error("missing empty state")
	}
}

func TestIncidentList(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	_, err := st.CreateIncident(context.Background(), incident.Incident{
		Title: "Database failover", Status: incident.StatusMonitoring, Impact: incident.ImpactMajor,
		StartedAt: time.Now().Add(-time.Hour),
	}, "Failing over.")
	if err != nil {
		t.Fatal(err)
	}
	c, _ := login(t, srv)
	body := getWith(t, srv, "/admin/incidents", c).Body.String()
	for _, want := range []string{"Database failover", "monitoring", "major"} {
		if !strings.Contains(body, want) {
			t.Errorf("list is missing %q", want)
		}
	}
}

func TestCreateIncident(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, csrf := login(t, srv)

	comp, err := st.CreateComponent(context.Background(), store.Component{Name: "Payments"})
	if err != nil {
		t.Fatal(err)
	}
	rec := postForm(t, srv, "/admin/incidents", url.Values{
		"csrf": {csrf}, "title": {"Payments delayed"}, "status": {"identified"},
		"impact": {"major"}, "message": {"Our payment provider is degraded."},
		"component_id": {itoa(comp.ID)},
	}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	if list, _ := st.ListIncidents(context.Background(), 1, 0); list[0].ComponentID != comp.ID {
		t.Errorf("component not saved: %+v", list[0])
	}
	body := getWith(t, srv, rec.Header().Get("Location"), c).Body.String()
	for _, want := range []string{"Payments delayed", "identified", "Our payment provider is degraded."} {
		if !strings.Contains(body, want) {
			t.Errorf("detail page is missing %q", want)
		}
	}
}

func TestCreateIncidentValidation(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, csrf := login(t, srv)
	rec := postForm(t, srv, "/admin/incidents", url.Values{
		"csrf": {csrf}, "title": {""}, "status": {"investigating"}, "impact": {"catastrophic"}, "message": {"x"},
	}, c)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "Title is required") || !strings.Contains(body, "Impact is not a valid impact") {
		t.Error("missing validation messages")
	}
	if n, _ := st.CountIncidents(context.Background()); n != 0 {
		t.Error("invalid incident saved")
	}
}

func TestAddIncidentUpdate(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	ctx := context.Background()
	inc, err := st.CreateIncident(ctx, incident.Incident{Title: "API errors", Status: incident.StatusInvestigating, Impact: incident.ImpactMinor}, "Looking.")
	if err != nil {
		t.Fatal(err)
	}
	c, csrf := login(t, srv)
	path := "/admin/incidents/" + itoa(inc.ID)

	rec := postForm(t, srv, path+"/updates", url.Values{"csrf": {csrf}, "status": {"resolved"}, "message": {"Fixed.\nThanks for your patience."}}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update: %d", rec.Code)
	}
	got, _ := st.GetIncident(ctx, inc.ID)
	if got.Status != incident.StatusResolved || len(got.Updates) != 2 {
		t.Fatalf("incident = %+v", got)
	}
	if body := getWith(t, srv, path, c).Body.String(); !strings.Contains(body, "Fixed.<br>Thanks for your patience.") {
		t.Error("line break not kept")
	}

	rec = postForm(t, srv, path+"/updates", url.Values{"csrf": {csrf}, "status": {"resolved"}, "message": {"  "}}, c)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Message is required") {
		t.Errorf("empty update: %d", rec.Code)
	}
}

func TestIncidentDetailNotFound(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, _ := login(t, srv)
	if rec := getWith(t, srv, "/admin/incidents/77", c); rec.Code != http.StatusNotFound {
		t.Errorf("got %d", rec.Code)
	}
}
