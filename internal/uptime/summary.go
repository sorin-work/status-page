package uptime

import (
	"context"
	"time"

	"github.com/sorin-work/status-page/internal/store"
)

// HistoryDays is how far back the long range uptime and the status page
// bars go.
const HistoryDays = 90

// Source is the data needed to compute uptime.
type Source interface {
	CountChecksSince(ctx context.Context, monitorID int64, since time.Time) (up, total int, err error)
	DailyStats(ctx context.Context, monitorID int64, from, to time.Time) ([]store.DailyStat, error)
}

// Summary is the uptime of one monitor over the standard periods.
type Summary struct {
	Day     Ratio // last 24 hours
	Week    Ratio // last 7 days
	Quarter Ratio // last 90 days
}

// Summarize computes the uptime summary of a monitor at now. The 24 hour
// and 7 day figures come from raw checks, the 90 day one from the daily
// rollups.
func Summarize(ctx context.Context, src Source, monitorID int64, now time.Time) (Summary, error) {
	var sum Summary
	up, total, err := src.CountChecksSince(ctx, monitorID, now.Add(-24*time.Hour))
	if err != nil {
		return sum, err
	}
	sum.Day = Ratio{Up: up, Total: total}

	up, total, err = src.CountChecksSince(ctx, monitorID, now.Add(-7*24*time.Hour))
	if err != nil {
		return sum, err
	}
	sum.Week = Ratio{Up: up, Total: total}

	stats, err := src.DailyStats(ctx, monitorID, now.AddDate(0, 0, -(HistoryDays-1)), now)
	if err != nil {
		return sum, err
	}
	for _, d := range stats {
		sum.Quarter = sum.Quarter.Add(Ratio{Up: d.Up, Total: d.Total})
	}
	return sum, nil
}

// BulkSource is the data needed to summarize all monitors at once.
type BulkSource interface {
	CountAllChecksSince(ctx context.Context, since time.Time) (map[int64]store.Counts, error)
	SumDailyStats(ctx context.Context, from, to time.Time) (map[int64]store.Counts, error)
}

// SummarizeAll computes the summary of every monitor with three queries,
// instead of three per monitor. Monitors without checks are missing from
// the map; their zero Summary means "no data".
func SummarizeAll(ctx context.Context, src BulkSource, now time.Time) (map[int64]Summary, error) {
	day, err := src.CountAllChecksSince(ctx, now.Add(-24*time.Hour))
	if err != nil {
		return nil, err
	}
	week, err := src.CountAllChecksSince(ctx, now.Add(-7*24*time.Hour))
	if err != nil {
		return nil, err
	}
	quarter, err := src.SumDailyStats(ctx, now.AddDate(0, 0, -(HistoryDays-1)), now)
	if err != nil {
		return nil, err
	}
	out := map[int64]Summary{}
	set := func(counts map[int64]store.Counts, apply func(*Summary, Ratio)) {
		for id, c := range counts {
			sum := out[id]
			apply(&sum, Ratio{Up: c.Up, Total: c.Total})
			out[id] = sum
		}
	}
	set(day, func(s *Summary, r Ratio) { s.Day = r })
	set(week, func(s *Summary, r Ratio) { s.Week = r })
	set(quarter, func(s *Summary, r Ratio) { s.Quarter = r })
	return out, nil
}
