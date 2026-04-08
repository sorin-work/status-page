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
	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/scheduler"
	"github.com/sorin-work/status-page/internal/store"
)

// Store is the persistence the manager needs.
type Store interface {
	OpenIncidentForMonitor(ctx context.Context, monitorID int64) (incident.Incident, error)
	CreateIncident(ctx context.Context, inc incident.Incident, message string) (incident.Incident, error)
	AddIncidentUpdate(ctx context.Context, incidentID int64, status incident.Status, message string, at time.Time) (incident.Update, error)
}

// Manager opens an incident when a monitor goes down.
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
}

func (m *Manager) monitorDown(ctx context.Context, t scheduler.Transition) error {
	_, err := m.store.OpenIncidentForMonitor(ctx, t.Monitor.ID)
	if err == nil {
		return nil // already open, e.g. after a restart
	}
	if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	at := t.Result.CheckedAt
	if at.IsZero() {
		at = m.now()
	}
	inc := incident.Incident{
		Title:     t.Monitor.Name + " is down",
		Status:    incident.StatusInvestigating,
		Impact:    incident.ImpactMajor,
		MonitorID: t.Monitor.ID,
		Automatic: true,
		StartedAt: at,
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
