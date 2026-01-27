package uptime

import (
	"time"

	"github.com/sorin-work/status-page/internal/store"
)

// Level classifies a day on the status page.
type Level string

// Bar levels, from best to worst.
const (
	LevelNoData   Level = "nodata"
	LevelUp       Level = "up"
	LevelDegraded Level = "degraded"
	LevelDown     Level = "down"
)

// Thresholds used to color a day.
const (
	UpThreshold       = 99.9
	DegradedThreshold = 95.0
)

// Bar is one day on the status page.
type Bar struct {
	Day        time.Time
	Ratio      Ratio
	Level      Level
	AvgLatency time.Duration
}

// LevelFor classifies a ratio.
func LevelFor(r Ratio) Level {
	switch {
	case !r.HasData():
		return LevelNoData
	case r.Percent() >= UpThreshold:
		return LevelUp
	case r.Percent() >= DegradedThreshold:
		return LevelDegraded
	default:
		return LevelDown
	}
}

// Bars returns one bar per day for the n days ending on today (UTC),
// oldest first. Days without rollups get a no-data bar. Stats from several
// monitors may be passed; they are combined per day.
func Bars(stats []store.DailyStat, today time.Time, n int) []Bar {
	type agg struct {
		ratio      Ratio
		latencySum int64
	}
	byDay := map[string]*agg{}
	for _, d := range stats {
		a := byDay[d.Day]
		if a == nil {
			a = &agg{}
			byDay[d.Day] = a
		}
		a.ratio = a.ratio.Add(Ratio{Up: d.Up, Total: d.Total})
		a.latencySum += d.LatencySumMS
	}

	end := today.UTC()
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	bars := make([]Bar, 0, n)
	for i := n - 1; i >= 0; i-- {
		day := end.AddDate(0, 0, -i)
		b := Bar{Day: day, Level: LevelNoData}
		if a := byDay[day.Format(store.DayLayout)]; a != nil {
			b.Ratio = a.ratio
			b.Level = LevelFor(a.ratio)
			if a.ratio.Total > 0 {
				b.AvgLatency = time.Duration(a.latencySum/int64(a.ratio.Total)) * time.Millisecond
			}
		}
		bars = append(bars, b)
	}
	return bars
}
