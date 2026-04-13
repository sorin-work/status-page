package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/incident"
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

	rec := postForm(t, srv, "/admin/incidents", url.Values{
		"csrf": {csrf}, "title": {"Payments delayed"}, "status": {"identified"},
		"impact": {"major"}, "message": {"Our payment provider is degraded."},
	}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
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

func TestIncidentDetailNotFound(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, _ := login(t, srv)
	if rec := getWith(t, srv, "/admin/incidents/77", c); rec.Code != http.StatusNotFound {
		t.Errorf("got %d", rec.Code)
	}
}
