package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/maintenance"
)

func TestMaintenanceWindows(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	comp, _ := s.CreateComponent(ctx, Component{Name: "DB"})

	mk := func(title string, start, end time.Duration, compID int64) maintenance.Window {
		t.Helper()
		w, err := s.CreateMaintenance(ctx, maintenance.Window{Title: title, ComponentID: compID, StartsAt: now.Add(start), EndsAt: now.Add(end)})
		if err != nil {
			t.Fatal(err)
		}
		return w
	}
	past := mk("past", -5*time.Hour, -4*time.Hour, 0)
	mk("running", -time.Hour, time.Hour, comp.ID)
	mk("later", 24*time.Hour, 26*time.Hour, 0)

	current, err := s.ListCurrentMaintenance(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(current) != 2 || current[0].Title != "running" || current[0].ComponentID != comp.ID || current[1].Title != "later" {
		t.Errorf("current = %+v", current)
	}
	old, err := s.ListPastMaintenance(ctx, now, 10)
	if err != nil || len(old) != 1 || old[0].ID != past.ID {
		t.Errorf("past = %+v %v", old, err)
	}
	if err := s.DeleteMaintenance(ctx, past.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteMaintenance(ctx, past.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}
}

func TestEndMaintenance(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	w, err := s.CreateMaintenance(ctx, maintenance.Window{Title: "x", StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EndMaintenance(ctx, w.ID, now); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.ListCurrentMaintenance(ctx, now); len(list) != 0 {
		t.Errorf("window still current: %+v", list)
	}
	if err := s.EndMaintenance(ctx, w.ID, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("ending a finished window: %v", err)
	}
}
