package store

import (
	"context"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
)

// MaxErrorLength caps the error message stored with a check.
const MaxErrorLength = 500

// RecordCheck stores the result of a check.
func (s *Store) RecordCheck(ctx context.Context, r monitor.Result) error {
	errMsg := r.Error
	if len(errMsg) > MaxErrorLength {
		errMsg = errMsg[:MaxErrorLength]
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO checks
		(monitor_id, checked_at, up, latency_ms, status_code, error)
		VALUES (?, ?, ?, ?, ?, ?)`,
		r.MonitorID, r.CheckedAt.Unix(), r.Up, r.Latency.Milliseconds(), r.StatusCode, errMsg)
	return err
}

// ListChecks returns the check history of a monitor, newest first.
func (s *Store) ListChecks(ctx context.Context, monitorID int64, limit, offset int) ([]monitor.Result, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT monitor_id, checked_at, up, latency_ms, status_code, error
		FROM checks WHERE monitor_id = ?
		ORDER BY checked_at DESC, id DESC
		LIMIT ? OFFSET ?`, monitorID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []monitor.Result
	for rows.Next() {
		var (
			r         monitor.Result
			checkedAt int64
			latencyMS int64
		)
		if err := rows.Scan(&r.MonitorID, &checkedAt, &r.Up, &latencyMS, &r.StatusCode, &r.Error); err != nil {
			return nil, err
		}
		r.CheckedAt = time.Unix(checkedAt, 0).UTC()
		r.Latency = time.Duration(latencyMS) * time.Millisecond
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountChecks returns how many checks are stored for a monitor.
func (s *Store) CountChecks(ctx context.Context, monitorID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM checks WHERE monitor_id = ?`, monitorID).Scan(&n)
	return n, err
}
