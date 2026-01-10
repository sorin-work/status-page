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
