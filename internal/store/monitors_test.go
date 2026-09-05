package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
)

func sampleMonitor(name string) monitor.Monitor {
	return monitor.Monitor{
		Name:           name,
		Type:           monitor.TypeHTTP,
		Target:         "https://example.com",
		Interval:       time.Minute,
		Timeout:        10 * time.Second,
		ExpectedStatus: 200,
		Keyword:        "ok",
		Retries:        2,
	}
}

func TestCreateAndGetMonitor(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	created, err := s.CreateMonitor(ctx, sampleMonitor("Website"))
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.CreatedAt.IsZero() {
		t.Fatalf("missing id or timestamps: %+v", created)
	}

	got, err := s.GetMonitor(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Website" || got.Interval != time.Minute || got.Timeout != 10*time.Second ||
		got.ExpectedStatus != 200 || got.Keyword != "ok" || got.Retries != 2 || got.Status != monitor.StatusUnknown {
		t.Errorf("round trip mismatch: %+v", got)
	}
}

func TestGetMissingMonitor(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.GetMonitor(context.Background(), 42); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListMonitorsSortedByName(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, n := range []string{"beta", "Alpha", "gamma"} {
		if _, err := s.CreateMonitor(ctx, sampleMonitor(n)); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.ListMonitors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, m := range list {
		names = append(names, m.Name)
	}
	if len(names) != 3 || names[0] != "Alpha" || names[1] != "beta" || names[2] != "gamma" {
		t.Errorf("unexpected order: %v", names)
	}
}

func TestListActiveMonitorsSkipsPaused(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	paused := sampleMonitor("paused")
	paused.Paused = true
	if _, err := s.CreateMonitor(ctx, paused); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateMonitor(ctx, sampleMonitor("active")); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListActiveMonitors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "active" {
		t.Errorf("unexpected active monitors: %+v", list)
	}
}

func TestUpdateMonitor(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	m, err := s.CreateMonitor(ctx, sampleMonitor("old"))
	if err != nil {
		t.Fatal(err)
	}
	m.Name = "new"
	m.Interval = 5 * time.Minute
	m.Keyword = ""
	m.NoRedirects = true
	updated, err := s.UpdateMonitor(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "new" || updated.Interval != 5*time.Minute || updated.Keyword != "" || !updated.NoRedirects {
		t.Errorf("update not saved: %+v", updated)
	}

	m.ID = 999
	if _, err := s.UpdateMonitor(ctx, m); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteMonitor(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	m, err := s.CreateMonitor(ctx, sampleMonitor("doomed"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteMonitor(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetMonitor(ctx, m.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("monitor still exists: %v", err)
	}
	if err := s.DeleteMonitor(ctx, m.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: expected ErrNotFound, got %v", err)
	}
}

func TestSetMonitorStatus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	m, err := s.CreateMonitor(ctx, sampleMonitor("api"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetMonitorStatus(ctx, m.ID, monitor.StatusDown); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetMonitor(ctx, m.ID)
	if got.Status != monitor.StatusDown {
		t.Errorf("status = %q", got.Status)
	}
	if err := s.SetMonitorStatus(ctx, 404, monitor.StatusUp); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
