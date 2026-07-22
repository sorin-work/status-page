package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
)

type fakeStore struct {
	mu       sync.Mutex
	monitors []monitor.Monitor
	checks   []monitor.Result
	statuses map[int64]monitor.Status
}

func (f *fakeStore) ListActiveMonitors(context.Context) ([]monitor.Monitor, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]monitor.Monitor(nil), f.monitors...), nil
}

func (f *fakeStore) RecordCheck(_ context.Context, r monitor.Result) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checks = append(f.checks, r)
	return nil
}

func (f *fakeStore) SetMonitorStatus(_ context.Context, id int64, st monitor.Status) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.statuses == nil {
		f.statuses = map[int64]monitor.Status{}
	}
	f.statuses[id] = st
	for i := range f.monitors {
		if f.monitors[i].ID == id {
			f.monitors[i].Status = st
		}
	}
	return nil
}

func (f *fakeStore) checkCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.checks)
}

// scriptedChecker returns results from a per-monitor script; once the
// script runs out the last value is repeated.
type scriptedChecker struct {
	mu     sync.Mutex
	script map[int64][]bool
	calls  map[int64]int
}

func (c *scriptedChecker) Check(_ context.Context, m monitor.Monitor) monitor.Result {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.calls == nil {
		c.calls = map[int64]int{}
	}
	steps := c.script[m.ID]
	i := c.calls[m.ID]
	c.calls[m.ID]++
	up := true
	if len(steps) > 0 {
		if i >= len(steps) {
			i = len(steps) - 1
		}
		up = steps[i]
	}
	r := monitor.Result{MonitorID: m.ID, CheckedAt: time.Now(), Up: up}
	if !up {
		r.Error = "down"
	}
	return r
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newTestScheduler(store Store, checker monitor.Checker, workers int) (*Scheduler, *clock) {
	c := &clock{t: time.Date(2026, 1, 17, 10, 0, 0, 0, time.UTC)}
	s := New(store, checker, workers, nil)
	s.now = c.now
	return s, c
}

func mon(id int64, interval time.Duration, retries int) monitor.Monitor {
	return monitor.Monitor{ID: id, Name: "m", Type: monitor.TypeHTTP, Interval: interval, Timeout: time.Second, Retries: retries, Status: monitor.StatusUnknown}
}

func runOnce(s *Scheduler) {
	s.RunDue(context.Background())
	s.Wait()
}

func TestRunDueRespectsInterval(t *testing.T) {
	store := &fakeStore{monitors: []monitor.Monitor{mon(1, time.Minute, 0), mon(2, 5*time.Minute, 0)}}
	s, clk := newTestScheduler(store, &scriptedChecker{}, 4)

	runOnce(s)
	if n := store.checkCount(); n != 2 {
		t.Fatalf("first run: %d checks, want 2", n)
	}
	clk.advance(30 * time.Second)
	runOnce(s)
	if n := store.checkCount(); n != 2 {
		t.Fatalf("before interval: %d checks, want 2", n)
	}
	clk.advance(30 * time.Second)
	runOnce(s)
	if n := store.checkCount(); n != 3 {
		t.Fatalf("after 1m: %d checks, want 3", n)
	}
	clk.advance(4 * time.Minute)
	runOnce(s)
	if n := store.checkCount(); n != 5 {
		t.Fatalf("after 5m: %d checks, want 5", n)
	}
}

func TestTransitions(t *testing.T) {
	store := &fakeStore{monitors: []monitor.Monitor{mon(1, time.Minute, 0)}}
	checker := &scriptedChecker{script: map[int64][]bool{1: {true, false, false, true}}}
	s, clk := newTestScheduler(store, checker, 1)

	var got []string
	s.OnTransition = func(_ context.Context, tr Transition) {
		got = append(got, string(tr.From)+">"+string(tr.To))
	}
	for range 4 {
		runOnce(s)
		clk.advance(time.Minute)
	}
	want := []string{"unknown>up", "up>down", "down>up"}
	if len(got) != len(want) {
		t.Fatalf("transitions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("transitions = %v, want %v", got, want)
		}
	}
	if store.statuses[1] != monitor.StatusUp {
		t.Errorf("stored status = %q", store.statuses[1])
	}
}

func TestDownWithoutRetries(t *testing.T) {
	store := &fakeStore{monitors: []monitor.Monitor{mon(1, time.Minute, 0)}}
	checker := &scriptedChecker{script: map[int64][]bool{1: {false}}}
	s, _ := newTestScheduler(store, checker, 1)
	runOnce(s)
	if store.statuses[1] != monitor.StatusDown {
		t.Errorf("status = %q, want down after the first failure", store.statuses[1])
	}
}

func TestJitterDelaysFirstChecks(t *testing.T) {
	var monitors []monitor.Monitor
	for i := int64(1); i <= 20; i++ {
		monitors = append(monitors, mon(i, time.Hour, 0))
	}
	store := &fakeStore{monitors: monitors}
	s, clk := newTestScheduler(store, &scriptedChecker{}, 4)
	s.Jitter = 10 * time.Second

	runOnce(s)
	if n := store.checkCount(); n == 20 {
		t.Fatal("all monitors ran immediately despite jitter")
	}
	clk.advance(10 * time.Second)
	runOnce(s)
	if n := store.checkCount(); n != 20 {
		t.Fatalf("after the jitter window %d of 20 monitors ran", n)
	}
}

func TestRetriesBeforeDown(t *testing.T) {
	store := &fakeStore{monitors: []monitor.Monitor{mon(1, time.Minute, 2)}}
	checker := &scriptedChecker{script: map[int64][]bool{1: {true, false, false, false}}}
	s, clk := newTestScheduler(store, checker, 1)

	want := []monitor.Status{monitor.StatusUp, monitor.StatusUp, monitor.StatusUp, monitor.StatusDown}
	for i, w := range want {
		runOnce(s)
		clk.advance(time.Minute)
		if got := store.statuses[1]; got != w {
			t.Fatalf("after check %d: status %q, want %q", i+1, got, w)
		}
	}
}

func TestRecoveryResetsRetries(t *testing.T) {
	store := &fakeStore{monitors: []monitor.Monitor{mon(1, time.Minute, 1)}}
	checker := &scriptedChecker{script: map[int64][]bool{1: {true, false, true, false, true}}}
	s, clk := newTestScheduler(store, checker, 1)
	for range 5 {
		runOnce(s)
		clk.advance(time.Minute)
		if store.statuses[1] == monitor.StatusDown {
			t.Fatal("isolated failures marked the monitor down")
		}
	}
}

type slowChecker struct {
	active, peak atomic.Int32
}

func (c *slowChecker) Check(_ context.Context, m monitor.Monitor) monitor.Result {
	n := c.active.Add(1)
	for {
		p := c.peak.Load()
		if n <= p || c.peak.CompareAndSwap(p, n) {
			break
		}
	}
	time.Sleep(20 * time.Millisecond)
	c.active.Add(-1)
	return monitor.Result{MonitorID: m.ID, Up: true}
}

func TestWorkerLimit(t *testing.T) {
	var monitors []monitor.Monitor
	for i := int64(1); i <= 6; i++ {
		monitors = append(monitors, mon(i, time.Minute, 0))
	}
	store := &fakeStore{monitors: monitors}
	checker := &slowChecker{}
	s, _ := newTestScheduler(store, checker, 2)
	runOnce(s)
	if store.checkCount() != 6 {
		t.Fatalf("ran %d checks, want 6", store.checkCount())
	}
	if p := checker.peak.Load(); p > 2 {
		t.Errorf("%d checks ran at once, limit is 2", p)
	}
}

// ctxChecker fails when its context is cancelled during the check.
type ctxChecker struct{ started chan struct{} }

func (c *ctxChecker) Check(ctx context.Context, m monitor.Monitor) monitor.Result {
	close(c.started)
	select {
	case <-ctx.Done():
		return monitor.Result{MonitorID: m.ID, Error: ctx.Err().Error()}
	case <-time.After(50 * time.Millisecond):
		return monitor.Result{MonitorID: m.ID, Up: true}
	}
}

func TestShutdownLetsRunningChecksFinish(t *testing.T) {
	store := &fakeStore{monitors: []monitor.Monitor{mon(1, time.Minute, 0)}}
	checker := &ctxChecker{started: make(chan struct{})}
	s, _ := newTestScheduler(store, checker, 1)
	ctx, cancel := context.WithCancel(context.Background())
	s.RunDue(ctx)
	<-checker.started
	cancel()
	s.Wait()
	if len(store.checks) != 1 || !store.checks[0].Up {
		t.Fatalf("check interrupted by shutdown: %+v", store.checks)
	}
	if store.statuses[1] == monitor.StatusDown {
		t.Error("shutdown marked the monitor down")
	}
}

// blockingChecker holds every check until release is closed.
type blockingChecker struct {
	calls   atomic.Int32
	release chan struct{}
}

func (c *blockingChecker) Check(_ context.Context, m monitor.Monitor) monitor.Result {
	c.calls.Add(1)
	<-c.release
	return monitor.Result{MonitorID: m.ID, Error: "timeout"}
}

func TestSlowCheckIsNotStartedTwice(t *testing.T) {
	store := &fakeStore{monitors: []monitor.Monitor{mon(1, 10*time.Second, 0)}}
	checker := &blockingChecker{release: make(chan struct{})}
	s, clk := newTestScheduler(store, checker, 4)
	var transitions atomic.Int32
	s.OnTransition = func(context.Context, Transition) { transitions.Add(1) }

	ctx := context.Background()
	// The check hangs for longer than the interval; the scheduler keeps
	// ticking meanwhile.
	for range 5 {
		s.RunDue(ctx)
		clk.advance(15 * time.Second)
	}
	time.Sleep(20 * time.Millisecond)
	close(checker.release)
	s.Wait()

	if n := checker.calls.Load(); n != 1 {
		t.Fatalf("%d checks started for one monitor, want 1", n)
	}
	if n := transitions.Load(); n != 1 {
		t.Errorf("%d transitions, want 1", n)
	}

	// Once it has finished the monitor is due again.
	runOnce(s)
	if n := checker.calls.Load(); n != 2 {
		t.Errorf("monitor not checked again after the slow check: %d", n)
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	store := &fakeStore{monitors: []monitor.Monitor{mon(1, time.Minute, 0)}}
	s, _ := newTestScheduler(store, &scriptedChecker{}, 1)
	s.Tick = 5 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
