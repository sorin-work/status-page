package store

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/sorin-work/status-page/internal/monitor"
)

// MaxErrorLength caps the error message stored with a check.
const MaxErrorLength = 500

// truncate shortens s to at most n bytes without cutting a UTF-8
// character in half.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// scanCheck reads a row of monitor_id, checked_at, up, latency_ms,
// status_code, error.
func scanCheck(r rowScanner) (monitor.Result, error) {
	var (
		res       monitor.Result
		checkedAt int64
		latencyMS int64
	)
	if err := r.Scan(&res.MonitorID, &checkedAt, &res.Up, &latencyMS, &res.StatusCode, &res.Error); err != nil {
		return monitor.Result{}, err
	}
	res.CheckedAt = time.Unix(checkedAt, 0).UTC()
	res.Latency = time.Duration(latencyMS) * time.Millisecond
	return res, nil
}

// RecordCheck stores the result of a check.
func (s *Store) RecordCheck(ctx context.Context, r monitor.Result) error {
	return s.RecordChecks(ctx, []monitor.Result{r})
}

// RecordChecks stores several results in one transaction and keeps the
// daily rollups in step.
func (s *Store) RecordChecks(ctx context.Context, results []monitor.Result) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	insertCheck, err := tx.PrepareContext(ctx, `INSERT INTO checks
		(monitor_id, checked_at, up, latency_ms, status_code, error)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insertCheck.Close()
	upsertDay, err := tx.PrepareContext(ctx, `INSERT INTO daily_stats (monitor_id, day, total, up, latency_sum_ms)
		VALUES (?, ?, 1, ?, ?)
		ON CONFLICT (monitor_id, day) DO UPDATE SET
			total = total + 1,
			up = up + excluded.up,
			latency_sum_ms = latency_sum_ms + excluded.latency_sum_ms`)
	if err != nil {
		return err
	}
	defer upsertDay.Close()

	for _, r := range results {
		errMsg := truncate(r.Error, MaxErrorLength)
		if _, err := insertCheck.ExecContext(ctx, r.MonitorID, r.CheckedAt.Unix(), r.Up,
			r.Latency.Milliseconds(), r.StatusCode, errMsg); err != nil {
			return err
		}
		up := 0
		if r.Up {
			up = 1
		}
		if _, err := upsertDay.ExecContext(ctx, r.MonitorID, Day(r.CheckedAt), up, r.Latency.Milliseconds()); err != nil {
			return err
		}
	}
	return tx.Commit()
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
		r, err := scanCheck(rows)
		if err != nil {
			return nil, err
		}
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

// CountChecksSince returns how many checks of a monitor ran at or after
// since, and how many of them succeeded.
func (s *Store) CountChecksSince(ctx context.Context, monitorID int64, since time.Time) (up, total int, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(up), 0), COUNT(*)
		FROM checks WHERE monitor_id = ? AND checked_at >= ?`, monitorID, since.Unix()).Scan(&up, &total)
	return up, total, err
}

// LatestChecks returns the most recent check of every monitor that has one.
func (s *Store) LatestChecks(ctx context.Context) (map[int64]monitor.Result, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.monitor_id, c.checked_at, c.up, c.latency_ms, c.status_code, c.error
		FROM checks c
		JOIN (SELECT monitor_id, MAX(id) AS id FROM checks GROUP BY monitor_id) last ON last.id = c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]monitor.Result{}
	for rows.Next() {
		r, err := scanCheck(rows)
		if err != nil {
			return nil, err
		}
		out[r.MonitorID] = r
	}
	return out, rows.Err()
}

// PruneChecks deletes raw checks older than before. Daily rollups are
// kept, so long range uptime is not affected.
func (s *Store) PruneChecks(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM checks WHERE checked_at < ?`, before.Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Counts is the number of checks and successful checks of a monitor.
type Counts struct {
	Up    int
	Total int
}

// CountAllChecksSince is CountChecksSince for every monitor in one query.
func (s *Store) CountAllChecksSince(ctx context.Context, since time.Time) (map[int64]Counts, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT monitor_id, COALESCE(SUM(up), 0), COUNT(*)
		FROM checks WHERE checked_at >= ? GROUP BY monitor_id`, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]Counts{}
	for rows.Next() {
		var (
			id int64
			c  Counts
		)
		if err := rows.Scan(&id, &c.Up, &c.Total); err != nil {
			return nil, err
		}
		out[id] = c
	}
	return out, rows.Err()
}
