package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
)

const monitorColumns = `id, name, type, target, interval_seconds, timeout_seconds,
	expected_status, keyword, retries, paused, status, created_at, updated_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanMonitor(r rowScanner) (monitor.Monitor, error) {
	var (
		m                 monitor.Monitor
		typ, status       string
		interval, timeout int64
		created, updated  int64
	)
	err := r.Scan(&m.ID, &m.Name, &typ, &m.Target, &interval, &timeout,
		&m.ExpectedStatus, &m.Keyword, &m.Retries, &m.Paused, &status, &created, &updated)
	if err != nil {
		return monitor.Monitor{}, err
	}
	m.Type = monitor.Type(typ)
	m.Status = monitor.Status(status)
	m.Interval = time.Duration(interval) * time.Second
	m.Timeout = time.Duration(timeout) * time.Second
	m.CreatedAt = time.Unix(created, 0).UTC()
	m.UpdatedAt = time.Unix(updated, 0).UTC()
	return m, nil
}

// CreateMonitor inserts m and returns it with its ID and timestamps set.
func (s *Store) CreateMonitor(ctx context.Context, m monitor.Monitor) (monitor.Monitor, error) {
	now := s.now().UTC().Truncate(time.Second)
	m.CreatedAt, m.UpdatedAt = now, now
	if m.Status == "" {
		m.Status = monitor.StatusUnknown
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO monitors
		(name, type, target, interval_seconds, timeout_seconds, expected_status, keyword, retries, paused, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.Name, string(m.Type), m.Target, int64(m.Interval/time.Second), int64(m.Timeout/time.Second),
		m.ExpectedStatus, m.Keyword, m.Retries, m.Paused, string(m.Status), now.Unix(), now.Unix())
	if err != nil {
		return monitor.Monitor{}, err
	}
	m.ID, err = res.LastInsertId()
	return m, err
}

// GetMonitor returns the monitor with the given ID.
func (s *Store) GetMonitor(ctx context.Context, id int64) (monitor.Monitor, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+monitorColumns+` FROM monitors WHERE id = ?`, id)
	m, err := scanMonitor(row)
	if errors.Is(err, sql.ErrNoRows) {
		return monitor.Monitor{}, ErrNotFound
	}
	return m, err
}

// ListMonitors returns every monitor ordered by name.
func (s *Store) ListMonitors(ctx context.Context) ([]monitor.Monitor, error) {
	return s.queryMonitors(ctx, `SELECT `+monitorColumns+` FROM monitors ORDER BY name COLLATE NOCASE, id`)
}

// ListActiveMonitors returns the monitors that are not paused.
func (s *Store) ListActiveMonitors(ctx context.Context) ([]monitor.Monitor, error) {
	return s.queryMonitors(ctx, `SELECT `+monitorColumns+` FROM monitors WHERE paused = 0 ORDER BY id`)
}

func (s *Store) queryMonitors(ctx context.Context, query string, args ...any) ([]monitor.Monitor, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []monitor.Monitor
	for rows.Next() {
		m, err := scanMonitor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpdateMonitor saves the editable fields of m. The status is left alone.
func (s *Store) UpdateMonitor(ctx context.Context, m monitor.Monitor) (monitor.Monitor, error) {
	now := s.now().UTC().Truncate(time.Second)
	res, err := s.db.ExecContext(ctx, `UPDATE monitors SET
		name = ?, type = ?, target = ?, interval_seconds = ?, timeout_seconds = ?,
		expected_status = ?, keyword = ?, retries = ?, paused = ?, updated_at = ?
		WHERE id = ?`,
		m.Name, string(m.Type), m.Target, int64(m.Interval/time.Second), int64(m.Timeout/time.Second),
		m.ExpectedStatus, m.Keyword, m.Retries, m.Paused, now.Unix(), m.ID)
	if err != nil {
		return monitor.Monitor{}, err
	}
	if err := expectOneRow(res); err != nil {
		return monitor.Monitor{}, err
	}
	return s.GetMonitor(ctx, m.ID)
}

// DeleteMonitor removes a monitor and its check history.
func (s *Store) DeleteMonitor(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM monitors WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return expectOneRow(res)
}

func expectOneRow(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
