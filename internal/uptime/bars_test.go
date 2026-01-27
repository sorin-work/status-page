package uptime

import (
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/store"
)

func TestLevelFor(t *testing.T) {
	cases := []struct {
		r    Ratio
		want Level
	}{
		{Ratio{}, LevelNoData},
		{Ratio{Up: 1000, Total: 1000}, LevelUp},
		{Ratio{Up: 999, Total: 1000}, LevelUp},
		{Ratio{Up: 990, Total: 1000}, LevelDegraded},
		{Ratio{Up: 950, Total: 1000}, LevelDegraded},
		{Ratio{Up: 949, Total: 1000}, LevelDown},
		{Ratio{Up: 0, Total: 3}, LevelDown},
	}
	for _, tc := range cases {
		if got := LevelFor(tc.r); got != tc.want {
			t.Errorf("%+v: got %s, want %s", tc.r, got, tc.want)
		}
	}
}

func TestBarsFillMissingDays(t *testing.T) {
	today := time.Date(2026, 1, 27, 18, 30, 0, 0, time.UTC)
	stats := []store.DailyStat{
		{Day: "2026-01-27", Total: 10, Up: 10, LatencySumMS: 1000},
		{Day: "2026-01-25", Total: 10, Up: 5, LatencySumMS: 2000},
		{Day: "2025-12-01", Total: 10, Up: 0}, // outside the window
	}
	bars := Bars(stats, today, 5)
	if len(bars) != 5 {
		t.Fatalf("got %d bars", len(bars))
	}
	wantDays := []string{"2026-01-23", "2026-01-24", "2026-01-25", "2026-01-26", "2026-01-27"}
	wantLevels := []Level{LevelNoData, LevelNoData, LevelDown, LevelNoData, LevelUp}
	for i, b := range bars {
		if got := b.Day.Format(store.DayLayout); got != wantDays[i] {
			t.Errorf("bar %d day = %s, want %s", i, got, wantDays[i])
		}
		if b.Level != wantLevels[i] {
			t.Errorf("bar %d level = %s, want %s", i, b.Level, wantLevels[i])
		}
	}
	if bars[4].AvgLatency != 100*time.Millisecond || bars[2].AvgLatency != 200*time.Millisecond {
		t.Errorf("latencies: %v %v", bars[4].AvgLatency, bars[2].AvgLatency)
	}
}

func TestBarsCombineMonitors(t *testing.T) {
	today := time.Date(2026, 1, 27, 0, 0, 0, 0, time.UTC)
	stats := []store.DailyStat{
		{MonitorID: 1, Day: "2026-01-27", Total: 100, Up: 100},
		{MonitorID: 2, Day: "2026-01-27", Total: 100, Up: 96},
	}
	bars := Bars(stats, today, 1)
	if bars[0].Ratio != (Ratio{Up: 196, Total: 200}) || bars[0].Level != LevelDegraded {
		t.Errorf("combined bar = %+v", bars[0])
	}
}
