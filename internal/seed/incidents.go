package seed

import (
	"context"
	"time"

	"github.com/sorin-work/status-page/internal/incident"
	"github.com/sorin-work/status-page/internal/maintenance"
	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/store"
)

type demoUpdate struct {
	after   time.Duration
	status  incident.Status
	message string
}

type demoIncident struct {
	title     string
	component string
	monitor   string
	impact    incident.Impact
	daysAgo   int
	hour      int
	updates   []demoUpdate
}

var demoIncidents = []demoIncident{
	{
		title: "Checkout is down", component: "Checkout", monitor: "Checkout", impact: incident.ImpactMajor, daysAgo: 3, hour: 14,
		updates: []demoUpdate{
			{0, incident.StatusInvestigating, "Checks are failing: expected status 200, got 502."},
			{12 * time.Minute, incident.StatusIdentified, "The payment gateway is rejecting connections after a certificate rotation.\nWe are rolling the change back."},
			{40 * time.Minute, incident.StatusResolved, "Checkout is responding again. Total downtime: 40m."},
		},
	},
	{
		title: "Elevated API error rates", component: "API", impact: incident.ImpactMinor, daysAgo: 12, hour: 9,
		updates: []demoUpdate{
			{0, incident.StatusInvestigating, "Some API requests are failing with 503 errors. We are looking into it."},
			{25 * time.Minute, incident.StatusMonitoring, "A bad cache node was taken out of rotation. Error rates are back to normal and we are monitoring."},
			{90 * time.Minute, incident.StatusResolved, "This incident has been resolved."},
		},
	},
	{
		title: "Database failover", component: "Database", impact: incident.ImpactMajor, daysAgo: 23, hour: 3,
		updates: []demoUpdate{
			{0, incident.StatusInvestigating, "The primary database stopped accepting connections."},
			{18 * time.Minute, incident.StatusResolved, "Traffic was moved to the replica. Writes were unavailable for about 20 minutes."},
		},
	},
	{
		title: "Slow search results", component: "Search", impact: incident.ImpactMinor, daysAgo: 40, hour: 16,
		updates: []demoUpdate{
			{0, incident.StatusIdentified, "Search is slower than usual while the index is rebuilt."},
			{3 * time.Hour, incident.StatusResolved, "The rebuild finished and search times are back to normal."},
		},
	},
}

// incidentsAndMaintenance adds resolved incidents that match the generated
// history, one past and one upcoming maintenance window.
func incidentsAndMaintenance(ctx context.Context, st *store.Store, components map[string]int64, monitors []monitor.Monitor, now time.Time) error {
	monitorIDs := map[string]int64{}
	for _, m := range monitors {
		monitorIDs[m.Name] = m.ID
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	for _, d := range demoIncidents {
		start := today.AddDate(0, 0, -d.daysAgo).Add(time.Duration(d.hour) * time.Hour)
		inc, err := st.CreateIncident(ctx, incident.Incident{
			Title:       d.title,
			Status:      d.updates[0].status,
			Impact:      d.impact,
			ComponentID: components[d.component],
			MonitorID:   monitorIDs[d.monitor],
			Automatic:   d.monitor != "",
			StartedAt:   start,
		}, d.updates[0].message)
		if err != nil {
			return err
		}
		for _, u := range d.updates[1:] {
			if _, err := st.AddIncidentUpdate(ctx, inc.ID, u.status, u.message, start.Add(u.after)); err != nil {
				return err
			}
		}
	}

	windows := []maintenance.Window{
		{
			Title:       "Search cluster upgrade",
			Description: "Search may return stale results for a few minutes while nodes restart.",
			ComponentID: components["Search"],
			StartsAt:    today.AddDate(0, 0, -16).Add(22 * time.Hour),
			EndsAt:      today.AddDate(0, 0, -16).Add(23 * time.Hour),
		},
		{
			Title:       "Database maintenance",
			Description: "We are upgrading the database cluster. Checkout and the API may be read-only for up to 30 minutes.",
			ComponentID: components["Database"],
			StartsAt:    today.AddDate(0, 0, 4).Add(2 * time.Hour),
			EndsAt:      today.AddDate(0, 0, 4).Add(2*time.Hour + 30*time.Minute),
		},
	}
	for _, w := range windows {
		if _, err := st.CreateMaintenance(ctx, w); err != nil {
			return err
		}
	}
	return nil
}
