package store

import (
	"context"
	"errors"
	"time"
)

// LatencyPoint is the response time of a monitor in one time bucket.
type LatencyPoint struct {
	Start    time.Time
	Avg      time.Duration
	Max      time.Duration
	Checks   int
	Failures int
}

// LatencySeries groups the checks of a monitor since a point in time into
// buckets of the given size. Only successful checks count towards the
// response times. Empty buckets are left out.
func (s *Store) LatencySeries(ctx context.Context, monitorID int64, since time.Time, bucket time.Duration) ([]LatencyPoint, error) {
	size := int64(bucket / time.Second)
	if size <= 0 {
		return nil, errors.New("bucket must be at least one second")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT (checked_at / ?) * ? AS bucket,
			COALESCE(AVG(CASE WHEN up = 1 THEN latency_ms END), 0),
			COALESCE(MAX(CASE WHEN up = 1 THEN latency_ms END), 0),
			COUNT(*),
			SUM(CASE WHEN up = 0 THEN 1 ELSE 0 END)
		FROM checks
		WHERE monitor_id = ? AND checked_at >= ?
		GROUP BY bucket
		ORDER BY bucket`, size, size, monitorID, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LatencyPoint
	for rows.Next() {
		var (
			p        LatencyPoint
			start    int64
			avg      float64
			maxMS    int64
			failures int
		)
		if err := rows.Scan(&start, &avg, &maxMS, &p.Checks, &failures); err != nil {
			return nil, err
		}
		p.Start = time.Unix(start, 0).UTC()
		p.Avg = time.Duration(avg * float64(time.Millisecond))
		p.Max = time.Duration(maxMS) * time.Millisecond
		p.Failures = failures
		out = append(out, p)
	}
	return out, rows.Err()
}
