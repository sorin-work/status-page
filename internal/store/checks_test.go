package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
)

func createMonitor(t *testing.T, s *Store, name string) monitor.Monitor {
	t.Helper()
	m, err := s.CreateMonitor(context.Background(), sampleMonitor(name))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func recordChecks(t *testing.T, s *Store, id int64, start time.Time, step time.Duration, ups ...bool) {
	t.Helper()
	for i, up := range ups {
		r := monitor.Result{MonitorID: id, CheckedAt: start.Add(time.Duration(i) * step), Up: up, Latency: time.Duration(100+i) * time.Millisecond}
		if !up {
			r.Error = "boom"
		}
		if err := s.RecordCheck(context.Background(), r); err != nil {
			t.Fatal(err)
		}
	}
}

func TestListChecksNewestFirst(t *testing.T) {
	s := newTestStore(t)
	m := createMonitor(t, s, "api")
	start := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	recordChecks(t, s, m.ID, start, time.Minute, true, false, true)

	checks, err := s.ListChecks(context.Background(), m.ID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 3 {
		t.Fatalf("got %d checks", len(checks))
	}
	if !checks[0].CheckedAt.Equal(start.Add(2*time.Minute)) || checks[1].Up || checks[1].Error != "boom" {
		t.Errorf("unexpected order or content: %+v", checks)
	}
	if checks[2].Latency != 100*time.Millisecond {
		t.Errorf("latency = %v", checks[2].Latency)
	}
}

func TestListChecksLimitOffset(t *testing.T) {
	s := newTestStore(t)
	m := createMonitor(t, s, "api")
	other := createMonitor(t, s, "other")
	start := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	recordChecks(t, s, m.ID, start, time.Minute, true, true, true, true, true)
	recordChecks(t, s, other.ID, start, time.Minute, false)

	page, err := s.ListChecks(context.Background(), m.ID, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || !page[0].CheckedAt.Equal(start.Add(2*time.Minute)) {
		t.Errorf("unexpected page: %+v", page)
	}
	n, err := s.CountChecks(context.Background(), m.ID)
	if err != nil || n != 5 {
		t.Errorf("CountChecks = %d, %v", n, err)
	}
}

func TestRecordCheckTruncatesLongErrors(t *testing.T) {
	s := newTestStore(t)
	m := createMonitor(t, s, "api")
	r := monitor.Result{MonitorID: m.ID, CheckedAt: time.Now(), Error: strings.Repeat("x", 2000)}
	if err := s.RecordCheck(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	checks, err := s.ListChecks(context.Background(), m.ID, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks[0].Error) != MaxErrorLength {
		t.Errorf("error length = %d", len(checks[0].Error))
	}
}

func TestDeletingMonitorRemovesChecks(t *testing.T) {
	s := newTestStore(t)
	m := createMonitor(t, s, "api")
	recordChecks(t, s, m.ID, time.Now(), time.Second, true, true)
	if err := s.DeleteMonitor(context.Background(), m.ID); err != nil {
		t.Fatal(err)
	}
	n, err := s.CountChecks(context.Background(), m.ID)
	if err != nil || n != 0 {
		t.Errorf("CountChecks after delete = %d, %v", n, err)
	}
}

func TestCountChecksSince(t *testing.T) {
	s := newTestStore(t)
	m := createMonitor(t, s, "api")
	start := time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)
	recordChecks(t, s, m.ID, start, time.Hour, false, true, true, false, true)

	up, total, err := s.CountChecksSince(context.Background(), m.ID, start.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if up != 2 || total != 3 {
		t.Errorf("up=%d total=%d, want 2/3", up, total)
	}
	up, total, err = s.CountChecksSince(context.Background(), m.ID, start.Add(10*time.Hour))
	if err != nil || up != 0 || total != 0 {
		t.Errorf("empty range: up=%d total=%d err=%v", up, total, err)
	}
}

func TestLatestChecks(t *testing.T) {
	s := newTestStore(t)
	a := createMonitor(t, s, "a")
	b := createMonitor(t, s, "b")
	createMonitor(t, s, "never checked")
	start := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	recordChecks(t, s, a.ID, start, time.Minute, true, true, false)
	recordChecks(t, s, b.ID, start, time.Minute, true)

	latest, err := s.LatestChecks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 2 {
		t.Fatalf("got %d entries", len(latest))
	}
	if r := latest[a.ID]; r.Up || !r.CheckedAt.Equal(start.Add(2*time.Minute)) {
		t.Errorf("latest of a = %+v", r)
	}
	if r := latest[b.ID]; !r.Up || r.Latency != 100*time.Millisecond {
		t.Errorf("latest of b = %+v", r)
	}
}

func TestPruneChecksKeepsRollups(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	m := createMonitor(t, s, "api")
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	recordChecks(t, s, m.ID, start, 24*time.Hour, true, false, true, true)

	n, err := s.PruneChecks(ctx, start.Add(48*time.Hour))
	if err != nil || n != 2 {
		t.Fatalf("pruned %d, %v", n, err)
	}
	if left, _ := s.CountChecks(ctx, m.ID); left != 2 {
		t.Errorf("%d checks left", left)
	}
	stats, _ := s.DailyStats(ctx, m.ID, start, start.AddDate(0, 0, 3))
	if len(stats) != 4 {
		t.Errorf("rollups were pruned too: %+v", stats)
	}
}
