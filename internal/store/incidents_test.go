package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/incident"
)

func newIncident(title string, started time.Time) incident.Incident {
	return incident.Incident{Title: title, Status: incident.StatusInvestigating, Impact: incident.ImpactMinor, StartedAt: started}
}

func TestCreateAndGetIncident(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	start := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)

	inc, err := s.CreateIncident(ctx, newIncident("  Slow logins ", start), "Looking into it.")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetIncident(ctx, inc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Slow logins" || got.Status != incident.StatusInvestigating || !got.StartedAt.Equal(start) || !got.ResolvedAt.IsZero() {
		t.Errorf("incident = %+v", got)
	}
	if len(got.Updates) != 1 || got.Updates[0].Message != "Looking into it." {
		t.Errorf("updates = %+v", got.Updates)
	}
	if _, err := s.GetIncident(ctx, 404); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestIncidentTimeline(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	start := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	inc, err := s.CreateIncident(ctx, newIncident("Errors", start), "Investigating.")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddIncidentUpdate(ctx, inc.ID, incident.StatusIdentified, "Found a bad deploy.", start.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddIncidentUpdate(ctx, inc.ID, incident.StatusResolved, "Rolled back.", start.Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetIncident(ctx, inc.ID)
	if got.Status != incident.StatusResolved || !got.ResolvedAt.Equal(start.Add(30*time.Minute)) {
		t.Errorf("not resolved: %+v", got)
	}
	if len(got.Updates) != 3 || got.Updates[0].Message != "Rolled back." || got.Updates[2].Message != "Investigating." {
		t.Errorf("timeline not newest first: %+v", got.Updates)
	}

	// Reopening clears the resolution time.
	if _, err := s.AddIncidentUpdate(ctx, inc.ID, incident.StatusMonitoring, "It is back.", start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetIncident(ctx, inc.ID)
	if !got.ResolvedAt.IsZero() || got.Status != incident.StatusMonitoring {
		t.Errorf("reopened incident = %+v", got)
	}

	if _, err := s.AddIncidentUpdate(ctx, 999, incident.StatusResolved, "x", time.Time{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListIncidentsNewestFirst(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	for i, title := range []string{"first", "second", "third"} {
		if _, err := s.CreateIncident(ctx, newIncident(title, start.AddDate(0, 0, i)), "msg"); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.ListIncidents(ctx, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Title != "third" || list[1].Title != "second" {
		t.Errorf("list = %+v", list)
	}
	if n, _ := s.CountIncidents(ctx); n != 3 {
		t.Errorf("count = %d", n)
	}
}

func TestOpenIncidentForMonitor(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	m := createMonitor(t, s, "api")
	if _, err := s.OpenIncidentForMonitor(ctx, m.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	inc := newIncident("api is down", time.Now())
	inc.MonitorID = m.ID
	inc.Automatic = true
	created, err := s.CreateIncident(ctx, inc, "down")
	if err != nil {
		t.Fatal(err)
	}
	open, err := s.OpenIncidentForMonitor(ctx, m.ID)
	if err != nil || open.ID != created.ID || open.MonitorID != m.ID || !open.Automatic {
		t.Fatalf("open incident = %+v, %v", open, err)
	}

	// Deleting the monitor keeps the incident but detaches it.
	if err := s.DeleteMonitor(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetIncident(ctx, created.ID)
	if got.MonitorID != 0 {
		t.Errorf("monitor_id = %d after delete", got.MonitorID)
	}
}
