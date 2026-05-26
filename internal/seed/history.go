package seed

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/store"
)

// rawDays is how many recent days get individual checks; older days only
// get daily rollups, like after the raw checks have been pruned.
const (
	rawDays     = 7
	rawSpacing  = 10 * time.Minute
	historyDays = 90
)

// profile describes how a demo monitor behaved in the past.
type profile struct {
	latency time.Duration
	// outages maps "days ago" to the share of failed checks that day.
	outages map[int]float64
}

var profiles = map[string]profile{
	"Storefront":    {latency: 120 * time.Millisecond, outages: map[int]float64{62: 0.02}},
	"API health":    {latency: 60 * time.Millisecond, outages: map[int]float64{51: 0.035, 12: 0.004}},
	"Checkout":      {latency: 250 * time.Millisecond, outages: map[int]float64{80: 0.12, 23: 0.06}},
	"Search":        {latency: 900 * time.Millisecond, outages: map[int]float64{40: 0.01}},
	"Database port": {latency: 2 * time.Millisecond, outages: map[int]float64{23: 0.03}},
}

// recentOutage is a short outage inside the raw check window, so the
// 24h/7d numbers and the response time chart have something to show.
type recentOutage struct {
	monitor  string
	daysAgo  int
	hour     int
	duration time.Duration
	err      string
}

var recentOutages = []recentOutage{
	{monitor: "Checkout", daysAgo: 3, hour: 14, duration: 40 * time.Minute, err: "expected status 200, got 502"},
}

// history writes rollups for older days and raw checks for the last week.
func history(ctx context.Context, st *store.Store, monitors []monitor.Monitor, now time.Time) error {
	rng := rand.New(rand.NewPCG(4417, 2026))
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	for _, m := range monitors {
		p, ok := profiles[m.Name]
		if !ok {
			continue
		}
		perDay := int((24 * time.Hour) / m.Interval)
		for ago := historyDays - 1; ago > rawDays; ago-- {
			day := today.AddDate(0, 0, -ago)
			failed := int(float64(perDay) * p.outages[ago])
			latency := jitter(rng, p.latency)
			if err := st.ImportDailyStat(ctx, store.DailyStat{
				MonitorID:    m.ID,
				Day:          store.Day(day),
				Total:        perDay,
				Up:           perDay - failed,
				LatencySumMS: latency.Milliseconds() * int64(perDay),
			}); err != nil {
				return err
			}
		}

		var batch []monitor.Result
		for t := now.Add(-rawDays * 24 * time.Hour); t.Before(now); t = t.Add(rawSpacing) {
			r := monitor.Result{MonitorID: m.ID, CheckedAt: t, Up: true, Latency: jitter(rng, p.latency)}
			if m.Type == monitor.TypeHTTP {
				r.StatusCode = m.ExpectedStatus
				if r.StatusCode == 0 {
					r.StatusCode = 200
				}
			}
			for _, o := range recentOutages {
				start := today.AddDate(0, 0, -o.daysAgo).Add(time.Duration(o.hour) * time.Hour)
				if o.monitor == m.Name && !t.Before(start) && t.Before(start.Add(o.duration)) {
					r.Up, r.Error, r.StatusCode = false, o.err, 502
				}
			}
			batch = append(batch, r)
		}
		if err := st.RecordChecks(ctx, batch); err != nil {
			return err
		}
	}
	return nil
}

// jitter varies a latency by up to ±30%.
func jitter(rng *rand.Rand, d time.Duration) time.Duration {
	f := 0.7 + rng.Float64()*0.6
	return time.Duration(float64(d) * f)
}
