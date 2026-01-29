package store

import (
	"context"
	"testing"
	"time"
)

func TestLatencySeries(t *testing.T) {
	s := newTestStore(t)
	m := createMonitor(t, s, "api")
	start := time.Date(2026, 1, 29, 10, 0, 0, 0, time.UTC)
	// 100..105 ms, one minute apart; the fourth check fails.
	recordChecks(t, s, m.ID, start, time.Minute, true, true, true, false, true, true)

	points, err := s.LatencySeries(context.Background(), m.ID, start, 3*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 2 {
		t.Fatalf("got %d points: %+v", len(points), points)
	}
	if !points[0].Start.Equal(start) || points[0].Avg != 101*time.Millisecond || points[0].Max != 102*time.Millisecond {
		t.Errorf("first bucket = %+v", points[0])
	}
	if points[1].Checks != 3 || points[1].Failures != 1 || points[1].Avg != 104500*time.Microsecond {
		t.Errorf("second bucket = %+v", points[1])
	}
}

func TestLatencySeriesRejectsTinyBuckets(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.LatencySeries(context.Background(), 1, time.Now(), time.Millisecond); err == nil {
		t.Fatal("expected an error")
	}
}
