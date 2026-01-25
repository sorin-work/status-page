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
