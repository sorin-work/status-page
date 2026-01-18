package store

import (
	"context"
	"testing"
	"time"
)

func TestDailyRollups(t *testing.T) {
	s := newTestStore(t)
	m := createMonitor(t, s, "api")
	// Two checks late on the 9th and three early on the 10th (UTC).
	start := time.Date(2026, 1, 9, 23, 58, 0, 0, time.UTC)
	recordChecks(t, s, m.ID, start, time.Minute, true, false, true, true, false)

	stats, err := s.DailyStats(context.Background(), m.ID, start.AddDate(0, 0, -1), start.AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("got %d days: %+v", len(stats), stats)
	}
	if stats[0].Day != "2026-01-09" || stats[0].Total != 2 || stats[0].Up != 1 {
		t.Errorf("first day = %+v", stats[0])
	}
	if stats[1].Day != "2026-01-10" || stats[1].Total != 3 || stats[1].Up != 2 {
		t.Errorf("second day = %+v", stats[1])
	}
	// Latencies are 100+i ms: 102, 103 and 104 on the second day.
	if got := stats[1].AvgLatency(); got != 103*time.Millisecond {
		t.Errorf("avg latency = %v", got)
	}
}

func TestDailyStatsRange(t *testing.T) {
	s := newTestStore(t)
	m := createMonitor(t, s, "api")
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	recordChecks(t, s, m.ID, start, 24*time.Hour, true, true, true, true, true)

	stats, err := s.DailyStats(context.Background(), m.ID, start.AddDate(0, 0, 1), start.AddDate(0, 0, 3))
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 3 || stats[0].Day != "2026-01-02" || stats[2].Day != "2026-01-04" {
		t.Errorf("unexpected range: %+v", stats)
	}
}

func TestDayUsesUTC(t *testing.T) {
	loc := time.FixedZone("UTC+2", 2*3600)
	if got := Day(time.Date(2026, 3, 1, 1, 0, 0, 0, loc)); got != "2026-02-28" {
		t.Errorf("Day = %s", got)
	}
	if (DailyStat{}).AvgLatency() != 0 {
		t.Error("empty day should have zero latency")
	}
}
