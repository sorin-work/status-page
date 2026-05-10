package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/sorin-work/status-page/internal/incident"
)

const incidentColumns = `i.id, i.title, i.status, i.impact, COALESCE(i.monitor_id, 0), i.automatic,
	i.started_at, COALESCE(i.resolved_at, 0), i.updated_at, COALESCE(i.component_id, 0)`

func scanIncident(r rowScanner) (incident.Incident, error) {
	var (
		inc                        incident.Incident
		status, impact             string
		started, resolved, updated int64
	)
	if err := r.Scan(&inc.ID, &inc.Title, &status, &impact, &inc.MonitorID, &inc.Automatic,
		&started, &resolved, &updated, &inc.ComponentID); err != nil {
		return incident.Incident{}, err
	}
	inc.Status = incident.Status(status)
	inc.Impact = incident.Impact(impact)
	inc.StartedAt = time.Unix(started, 0).UTC()
	if resolved > 0 {
		inc.ResolvedAt = time.Unix(resolved, 0).UTC()
	}
	inc.UpdatedAt = time.Unix(updated, 0).UTC()
	return inc, nil
}

func nullableID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

// CreateIncident opens an incident with its first timeline update.
func (s *Store) CreateIncident(ctx context.Context, inc incident.Incident, message string) (incident.Incident, error) {
	if inc.StartedAt.IsZero() {
		inc.StartedAt = s.now()
	}
	inc.StartedAt = inc.StartedAt.UTC().Truncate(time.Second)
	inc.UpdatedAt = inc.StartedAt
	var resolved any
	if inc.Status == incident.StatusResolved {
		if inc.ResolvedAt.IsZero() {
			inc.ResolvedAt = inc.StartedAt
		}
		resolved = inc.ResolvedAt.Unix()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return incident.Incident{}, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `INSERT INTO incidents
		(title, status, impact, monitor_id, automatic, started_at, resolved_at, updated_at, component_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(inc.Title), string(inc.Status), string(inc.Impact), nullableID(inc.MonitorID),
		inc.Automatic, inc.StartedAt.Unix(), resolved, inc.UpdatedAt.Unix(), nullableID(inc.ComponentID))
	if err != nil {
		return incident.Incident{}, err
	}
	if inc.ID, err = res.LastInsertId(); err != nil {
		return incident.Incident{}, err
	}
	u := incident.Update{IncidentID: inc.ID, Status: inc.Status, Message: strings.TrimSpace(message), CreatedAt: inc.StartedAt}
	if u.ID, err = insertUpdate(ctx, tx, u); err != nil {
		return incident.Incident{}, err
	}
	if err := tx.Commit(); err != nil {
		return incident.Incident{}, err
	}
	inc.Updates = []incident.Update{u}
	return inc, nil
}

func insertUpdate(ctx context.Context, tx *sql.Tx, u incident.Update) (int64, error) {
	res, err := tx.ExecContext(ctx, `INSERT INTO incident_updates (incident_id, status, message, created_at)
		VALUES (?, ?, ?, ?)`, u.IncidentID, string(u.Status), u.Message, u.CreatedAt.Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// AddIncidentUpdate appends to the timeline and moves the incident to the
// update's status. Resolving sets the resolution time; reopening clears it.
func (s *Store) AddIncidentUpdate(ctx context.Context, incidentID int64, status incident.Status, message string, at time.Time) (incident.Update, error) {
	if at.IsZero() {
		at = s.now()
	}
	at = at.UTC().Truncate(time.Second)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return incident.Update{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var resolved any
	if status == incident.StatusResolved {
		resolved = at.Unix()
	}
	res, err := tx.ExecContext(ctx, `UPDATE incidents SET status = ?, resolved_at = ?, updated_at = ? WHERE id = ?`,
		string(status), resolved, at.Unix(), incidentID)
	if err != nil {
		return incident.Update{}, err
	}
	if err := expectOneRow(res); err != nil {
		return incident.Update{}, err
	}
	u := incident.Update{IncidentID: incidentID, Status: status, Message: strings.TrimSpace(message), CreatedAt: at}
	if u.ID, err = insertUpdate(ctx, tx, u); err != nil {
		return incident.Update{}, err
	}
	return u, tx.Commit()
}

// GetIncident returns an incident with its timeline, newest update first.
func (s *Store) GetIncident(ctx context.Context, id int64) (incident.Incident, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+incidentColumns+` FROM incidents i WHERE i.id = ?`, id)
	inc, err := scanIncident(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incident.Incident{}, ErrNotFound
	}
	if err != nil {
		return incident.Incident{}, err
	}
	inc.Updates, err = s.incidentUpdates(ctx, id)
	return inc, err
}

func (s *Store) incidentUpdates(ctx context.Context, incidentID int64) ([]incident.Update, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, incident_id, status, message, created_at
		FROM incident_updates WHERE incident_id = ? ORDER BY created_at DESC, id DESC`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []incident.Update
	for rows.Next() {
		var (
			u       incident.Update
			status  string
			created int64
		)
		if err := rows.Scan(&u.ID, &u.IncidentID, &status, &u.Message, &created); err != nil {
			return nil, err
		}
		u.Status = incident.Status(status)
		u.CreatedAt = time.Unix(created, 0).UTC()
		out = append(out, u)
	}
	return out, rows.Err()
}

// ListIncidents returns incidents newest first, without their timelines.
func (s *Store) ListIncidents(ctx context.Context, limit, offset int) ([]incident.Incident, error) {
	return s.queryIncidents(ctx, `SELECT `+incidentColumns+` FROM incidents i
		ORDER BY i.started_at DESC, i.id DESC LIMIT ? OFFSET ?`, limit, offset)
}

// CountIncidents returns the number of incidents.
func (s *Store) CountIncidents(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM incidents`).Scan(&n)
	return n, err
}

// OpenIncidentForMonitor returns the unresolved automatic incident of a
// monitor, if there is one.
func (s *Store) OpenIncidentForMonitor(ctx context.Context, monitorID int64) (incident.Incident, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+incidentColumns+` FROM incidents i
		WHERE i.monitor_id = ? AND i.automatic = 1 AND i.status != 'resolved'
		ORDER BY i.started_at DESC LIMIT 1`, monitorID)
	inc, err := scanIncident(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incident.Incident{}, ErrNotFound
	}
	return inc, err
}

func (s *Store) queryIncidents(ctx context.Context, query string, args ...any) ([]incident.Incident, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []incident.Incident
	for rows.Next() {
		inc, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inc)
	}
	return out, rows.Err()
}

// ListOpenIncidents returns unresolved incidents with their timelines,
// newest first.
func (s *Store) ListOpenIncidents(ctx context.Context) ([]incident.Incident, error) {
	list, err := s.queryIncidents(ctx, `SELECT `+incidentColumns+` FROM incidents i
		WHERE i.status != 'resolved' ORDER BY i.started_at DESC, i.id DESC`)
	if err != nil {
		return nil, err
	}
	for k := range list {
		if list[k].Updates, err = s.incidentUpdates(ctx, list[k].ID); err != nil {
			return nil, err
		}
	}
	return list, nil
}

// ListRecentResolved returns the most recently started resolved incidents,
// up to limit, with their timelines.
func (s *Store) ListRecentResolved(ctx context.Context, limit int) ([]incident.Incident, error) {
	list, err := s.queryIncidents(ctx, `SELECT `+incidentColumns+` FROM incidents i
		WHERE i.status = 'resolved' ORDER BY i.started_at LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	for k := range list {
		if list[k].Updates, err = s.incidentUpdates(ctx, list[k].ID); err != nil {
			return nil, err
		}
	}
	return list, nil
}
