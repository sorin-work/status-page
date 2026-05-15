package store

import (
	"context"
	"strings"
	"time"

	"github.com/sorin-work/status-page/internal/maintenance"
)

const maintenanceColumns = `id, title, description, COALESCE(component_id, 0), starts_at, ends_at, created_at`

func scanWindow(r rowScanner) (maintenance.Window, error) {
	var (
		w                     maintenance.Window
		starts, ends, created int64
	)
	if err := r.Scan(&w.ID, &w.Title, &w.Description, &w.ComponentID, &starts, &ends, &created); err != nil {
		return maintenance.Window{}, err
	}
	w.StartsAt = time.Unix(starts, 0).UTC()
	w.EndsAt = time.Unix(ends, 0).UTC()
	w.CreatedAt = time.Unix(created, 0).UTC()
	return w, nil
}

// CreateMaintenance schedules a maintenance window.
func (s *Store) CreateMaintenance(ctx context.Context, w maintenance.Window) (maintenance.Window, error) {
	w.Title = strings.TrimSpace(w.Title)
	w.Description = strings.TrimSpace(w.Description)
	w.CreatedAt = s.now().UTC().Truncate(time.Second)
	res, err := s.db.ExecContext(ctx, `INSERT INTO maintenance (title, description, component_id, starts_at, ends_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		w.Title, w.Description, nullableID(w.ComponentID), w.StartsAt.Unix(), w.EndsAt.Unix(), w.CreatedAt.Unix())
	if err != nil {
		return maintenance.Window{}, err
	}
	w.ID, err = res.LastInsertId()
	return w, err
}

// ListCurrentMaintenance returns windows that have not ended at t, soonest
// first.
func (s *Store) ListCurrentMaintenance(ctx context.Context, t time.Time) ([]maintenance.Window, error) {
	return s.queryWindows(ctx, `SELECT `+maintenanceColumns+` FROM maintenance
		WHERE ends_at > ? ORDER BY starts_at, id`, t.Unix())
}

// ListPastMaintenance returns windows that ended before t, latest first.
func (s *Store) ListPastMaintenance(ctx context.Context, t time.Time, limit int) ([]maintenance.Window, error) {
	return s.queryWindows(ctx, `SELECT `+maintenanceColumns+` FROM maintenance
		WHERE ends_at <= ? ORDER BY ends_at DESC, id DESC LIMIT ?`, t.Unix(), limit)
}

// DeleteMaintenance removes a window.
func (s *Store) DeleteMaintenance(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM maintenance WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return expectOneRow(res)
}

func (s *Store) queryWindows(ctx context.Context, query string, args ...any) ([]maintenance.Window, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []maintenance.Window
	for rows.Next() {
		w, err := scanWindow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
