package web

import (
	"strings"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/store"
)

func TestBuildLatencyChart(t *testing.T) {
	from := time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC)
	to := from.Add(4 * time.Hour)
	points := []store.LatencyPoint{
		{Start: from, Avg: 100 * time.Millisecond, Checks: 4},
		{Start: from.Add(2 * time.Hour), Avg: 200 * time.Millisecond, Checks: 4, Failures: 1},
		{Start: from.Add(3 * time.Hour), Checks: 4, Failures: 4},
	}
	c := buildLatencyChart(points, from, to)
	if c.Empty || c.MaxLabel != "200ms" || c.AvgLabel != "150ms" {
		t.Errorf("labels: %+v", c)
	}
	coords := strings.Fields(c.Points)
	if len(coords) != 2 || coords[0] != "0.0,80.0" || coords[1] != "400.0,4.0" {
		t.Errorf("points = %q", c.Points)
	}
	if len(c.Marks) != 2 || c.Marks[1].X != 600 {
		t.Errorf("marks = %+v", c.Marks)
	}
}

func TestBuildLatencyChartSinglePoint(t *testing.T) {
	from := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	c := buildLatencyChart([]store.LatencyPoint{{Start: from.Add(time.Hour), Avg: 80 * time.Millisecond, Checks: 1}}, from, from.Add(2*time.Hour))
	if c.Empty || c.Dot == nil || c.Dot.X != 400 {
		t.Errorf("a single check should still be visible: %+v", c)
	}
}

func TestBuildLatencyChartEmpty(t *testing.T) {
	now := time.Now()
	c := buildLatencyChart(nil, now.Add(-time.Hour), now)
	if !c.Empty || c.Points != "" {
		t.Errorf("expected an empty chart: %+v", c)
	}
}
