package store

import (
	"context"
	"time"
)

// DayLayout is the format of the day column in daily_stats.
const DayLayout = "2006-01-02"

// Day returns the UTC calendar day of t as stored in daily_stats.
func Day(t time.Time) string {
	return t.UTC().Format(DayLayout)
}

// DailyStat aggregates the checks of one monitor on one UTC day.
type DailyStat struct {
	MonitorID    int64
	Day          string
	Total        int
	Up           int
	LatencySumMS int64
}

// AvgLatency returns the mean response time of the day.
func (d DailyStat) AvgLatency() time.Duration {
	if d.Total == 0 {
		return 0
	}
	return time.Duration(d.LatencySumMS/int64(d.Total)) * time.Millisecond
}

// DailyStats returns the rollups of a monitor between the two days
// (inclusive), oldest first. Days without checks are not returned.
func (s *Store) DailyStats(ctx context.Context, monitorID int64, from, to time.Time) ([]DailyStat, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT monitor_id, day, total, up, latency_sum_ms
		FROM daily_stats WHERE monitor_id = ? AND day >= ? AND day <= ?
		ORDER BY day`, monitorID, Day(from), Day(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DailyStat
	for rows.Next() {
		var d DailyStat
		if err := rows.Scan(&d.MonitorID, &d.Day, &d.Total, &d.Up, &d.LatencySumMS); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ComponentDailyStats returns the rollups of every monitor in a component
// between the two days (inclusive).
func (s *Store) ComponentDailyStats(ctx context.Context, componentID int64, from, to time.Time) ([]DailyStat, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT d.monitor_id, d.day, d.total, d.up, d.latency_sum_ms
		FROM daily_stats d JOIN monitors m ON m.id = d.monitor_id
		WHERE m.component_id = ? AND d.day >= ? AND d.day <= ?
		ORDER BY d.day, d.monitor_id`, componentID, Day(from), Day(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DailyStat
	for rows.Next() {
		var d DailyStat
		if err := rows.Scan(&d.MonitorID, &d.Day, &d.Total, &d.Up, &d.LatencySumMS); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ImportDailyStat writes a rollup directly. It is used to load history
// whose raw checks are not kept, such as the demo data.
func (s *Store) ImportDailyStat(ctx context.Context, d DailyStat) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO daily_stats (monitor_id, day, total, up, latency_sum_ms)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (monitor_id, day) DO UPDATE SET
			total = excluded.total, up = excluded.up, latency_sum_ms = excluded.latency_sum_ms`,
		d.MonitorID, d.Day, d.Total, d.Up, d.LatencySumMS)
	return err
}
