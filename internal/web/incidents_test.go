package web

import (
	"context"
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
