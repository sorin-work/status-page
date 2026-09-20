package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
)

// The scheduler workers and the web handlers share one store. Writes from
// many goroutines must neither fail with SQLITE_BUSY nor lose rows.
func TestConcurrentCheckWrites(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	var ids []int64
	for _, name := range []string{"a", "b", "c", "d"} {
		ids = append(ids, createMonitor(t, s, name).ID)
	}
	start := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

	var wg sync.WaitGroup
	errs := make(chan error, 400)
	for _, id := range ids {
		for w := range 4 {
			wg.Add(1)
			go func(id int64, w int) {
				defer wg.Done()
				for i := range 25 {
					r := monitor.Result{MonitorID: id, CheckedAt: start.Add(time.Duration(w*25+i) * time.Second), Up: i%5 != 0}
					if err := s.RecordCheck(ctx, r); err != nil {
						errs <- err
						return
					}
					if _, err := s.LatestChecks(ctx); err != nil {
						errs <- err
						return
					}
				}
			}(id, w)
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	for _, id := range ids {
		if n, _ := s.CountChecks(ctx, id); n != 100 {
			t.Errorf("monitor %d has %d checks, want 100", id, n)
		}
		stats, _ := s.DailyStats(ctx, id, start, start)
		if len(stats) != 1 || stats[0].Total != 100 || stats[0].Up != 80 {
			t.Errorf("rollup of %d = %+v", id, stats)
		}
	}
}
