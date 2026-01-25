package uptime

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/store"
)

func TestRatioString(t *testing.T) {
	cases := []struct {
		r    Ratio
		want string
	}{
		{Ratio{}, "no data"},
		{Ratio{Up: 1, Total: 1}, "100.00%"},
		{Ratio{Up: 999, Total: 1000}, "99.90%"},
		{Ratio{Up: 1, Total: 4}, "25.00%"},
		{Ratio{Up: 0, Total: 5}, "0.00%"},
	}
	for _, tc := range cases {
		if got := tc.r.String(); got != tc.want {
			t.Errorf("%+v: got %q, want %q", tc.r, got, tc.want)
		}
	}
}

func TestRatioPercentAndAdd(t *testing.T) {
	if p := (Ratio{}).Percent(); p != 100 {
		t.Errorf("empty ratio = %v", p)
	}
	r := Ratio{Up: 3, Total: 4}.Add(Ratio{Up: 1, Total: 4})
	if r.Up != 4 || r.Total != 8 || r.Percent() != 50 {
		t.Errorf("Add = %+v (%v%%)", r, r.Percent())
	}
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "uptime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSummarize(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	m, err := s.CreateMonitor(ctx, monitor.Monitor{Name: "api", Type: monitor.TypeHTTP, Target: "https://x.test", Interval: time.Minute, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 25, 12, 0, 0, 0, time.UTC)
	record := func(at time.Time, up bool) {
		t.Helper()
		if err := s.RecordCheck(ctx, monitor.Result{MonitorID: m.ID, CheckedAt: at, Up: up}); err != nil {
			t.Fatal(err)
		}
	}
	// Within 24h: 3 up, 1 down.
	for i := range 4 {
		record(now.Add(-time.Duration(i+1)*time.Hour), i != 0)
	}
	// 3 days ago: 1 down.
	record(now.Add(-72*time.Hour), false)
	// 30 days ago: 2 up. Only counts for 90 days.
	record(now.AddDate(0, 0, -30), true)
	record(now.AddDate(0, 0, -30).Add(time.Minute), true)
	// 120 days ago: ignored.
	record(now.AddDate(0, 0, -120), false)

	sum, err := Summarize(ctx, s, m.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Day != (Ratio{Up: 3, Total: 4}) {
		t.Errorf("24h = %+v", sum.Day)
	}
	if sum.Week != (Ratio{Up: 3, Total: 5}) {
		t.Errorf("7d = %+v", sum.Week)
	}
	if sum.Quarter != (Ratio{Up: 5, Total: 7}) {
		t.Errorf("90d = %+v", sum.Quarter)
	}
}
