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
