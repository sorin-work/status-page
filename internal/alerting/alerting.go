// Package alerting reacts to monitors changing state by opening and
// resolving incidents.
package alerting

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/sorin-work/status-page/internal/incident"
	"github.com/sorin-work/status-page/internal/maintenance"
	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/scheduler"
	"github.com/sorin-work/status-page/internal/store"
)

// Store is the persistence the manager needs.
type Store interface {
	OpenIncidentForMonitor(ctx context.Context, monitorID int64) (incident.Incident, error)
	CreateIncident(ctx context.Context, inc incident.Incident, message string) (incident.Incident, error)
	AddIncidentUpdate(ctx context.Context, incidentID int64, status incident.Status, message string, at time.Time) (incident.Update, error)
	ListCurrentMaintenance(ctx context.Context, t time.Time) ([]maintenance.Window, error)
}

// Manager opens an incident when a monitor goes down and resolves it when
// the monitor recovers.
type Manager struct {
	store  Store
	logger *slog.Logger
	now    func() time.Time
}

// New returns a Manager.
func New(st Store, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{store: st, logger: logger, now: time.Now}
}

// HandleTransition is meant to be used as the scheduler's OnTransition.
func (m *Manager) HandleTransition(ctx context.Context, t scheduler.Transition) {
	if t.To == monitor.StatusDown && t.From != monitor.StatusDown {
		if err := m.monitorDown(ctx, t); err != nil {
			m.logger.Error("open incident", "monitor", t.Monitor.ID, "err", err)
		}
	}
	if t.To == monitor.StatusUp && t.From == monitor.StatusDown {
		if err := m.monitorUp(ctx, t); err != nil {
			m.logger.Error("resolve incident", "monitor", t.Monitor.ID, "err", err)
		}
	}
}

func (m *Manager) monitorUp(ctx context.Context, t scheduler.Transition) error {
	inc, err := m.store.OpenIncidentForMonitor(ctx, t.Monitor.ID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	at := t.Result.CheckedAt
	if at.IsZero() {
		at = m.now()
	}
	msg := fmt.Sprintf("%s is responding again. Total downtime: %s.", t.Monitor.Name, humanDuration(at.Sub(inc.StartedAt)))
	if _, err := m.store.AddIncidentUpdate(ctx, inc.ID, incident.StatusResolved, msg, at); err != nil {
		return err
	}
	m.logger.Info("incident resolved", "incident", inc.ID, "monitor", t.Monitor.ID)
	return nil
}

// humanDuration rounds to minutes, e.g. "1h 5m" or "less than a minute".
func humanDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	if d < time.Minute {
		return "less than a minute"
	}
	h, m := int(d.Hours()), int(d.Minutes())%60
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

// inMaintenance reports whether a maintenance window covers the monitor.
func (m *Manager) inMaintenance(ctx context.Context, mon monitor.Monitor, at time.Time) (bool, error) {
	windows, err := m.store.ListCurrentMaintenance(ctx, at)
	if err != nil {
		return false, err
	}
	for _, w := range windows {
		if w.Active(at) && w.Covers(mon.ComponentID) {
			return true, nil
		}
	}
	return false, nil
}

func (m *Manager) monitorDown(ctx context.Context, t scheduler.Transition) error {
	at := t.Result.CheckedAt
	if at.IsZero() {
		at = m.now()
	}
	planned, err := m.inMaintenance(ctx, t.Monitor, at)
	if err != nil {
		return err
	}
	if planned {
		m.logger.Info("monitor down during maintenance, no incident opened", "monitor", t.Monitor.ID)
		return nil
	}
	_, err = m.store.OpenIncidentForMonitor(ctx, t.Monitor.ID)
	if err == nil {
		return nil // already open, e.g. after a restart
	}
	if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	inc := incident.Incident{
		Title:       t.Monitor.Name + " is down",
		Status:      incident.StatusInvestigating,
		Impact:      incident.ImpactMajor,
		MonitorID:   t.Monitor.ID,
		ComponentID: t.Monitor.ComponentID,
		Automatic:   true,
		StartedAt:   at,
	}
	msg := "Checks are failing."
	if t.Result.Error != "" {
		msg = fmt.Sprintf("Checks are failing: %s.", t.Result.Error)
	}
	created, err := m.store.CreateIncident(ctx, inc, msg)
	if err != nil {
		return err
	}
	m.logger.Info("incident opened", "incident", created.ID, "monitor", t.Monitor.ID)
	return nil
}
