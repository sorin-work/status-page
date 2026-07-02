// Package scheduler runs monitor checks on their configured intervals.
package scheduler

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
)

// Store is the persistence the scheduler needs.
type Store interface {
	ListActiveMonitors(ctx context.Context) ([]monitor.Monitor, error)
	RecordCheck(ctx context.Context, r monitor.Result) error
	SetMonitorStatus(ctx context.Context, id int64, status monitor.Status) error
}

// Transition describes a monitor changing state.
type Transition struct {
	Monitor monitor.Monitor
	From    monitor.Status
	To      monitor.Status
	Result  monitor.Result
}

// Scheduler decides which monitors are due and runs their checks.
type Scheduler struct {
	store   Store
	checker monitor.Checker
	logger  *slog.Logger
	now     func() time.Time

	// Tick is how often the scheduler looks for due monitors.
	Tick time.Duration
	// Jitter spreads the first check of each monitor over a random delay
	// of up to this long, so a restart does not fire every check at once.
	Jitter time.Duration
	// OnTransition, if set, is called after a monitor changes state.
	OnTransition func(ctx context.Context, t Transition)

	sem chan struct{}
	wg  sync.WaitGroup

	mu    sync.Mutex
	state map[int64]*monitorState
}

// monitorState is what the scheduler remembers about a monitor between
// checks.
type monitorState struct {
	lastRun  time.Time
	failures int
	status   monitor.Status
}

// DefaultWorkers is how many checks may run at the same time.
const DefaultWorkers = 8

// New returns a scheduler that uses store and checker.
func New(store Store, checker monitor.Checker, workers int, logger *slog.Logger) *Scheduler {
	if workers <= 0 {
		workers = DefaultWorkers
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		store:   store,
		checker: checker,
		logger:  logger,
		now:     time.Now,
		Tick:    time.Second,
		sem:     make(chan struct{}, workers),
		state:   map[int64]*monitorState{},
	}
}

// Run checks for due monitors every Tick until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	t := time.NewTicker(s.Tick)
	defer t.Stop()
	s.RunDue(ctx)
	for {
		select {
		case <-ctx.Done():
			s.Wait()
			return
		case <-t.C:
			s.RunDue(ctx)
		}
	}
}

// Wait blocks until every running check has finished.
func (s *Scheduler) Wait() {
	s.wg.Wait()
}

// RunDue starts the checks of every monitor whose interval has elapsed.
// Checks run in the background, at most Workers at a time.
func (s *Scheduler) RunDue(ctx context.Context) {
	monitors, err := s.store.ListActiveMonitors(ctx)
	if err != nil {
		s.logger.Error("list monitors", "err", err)
		return
	}
	now := s.now()
	for _, m := range monitors {
		if !s.isDue(m, now) {
			continue
		}
		s.wg.Add(1)
		go func(m monitor.Monitor) {
			defer s.wg.Done()
			select {
			case s.sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-s.sem }()
			s.runCheck(ctx, m)
		}(m)
	}
}

func (s *Scheduler) isDue(m monitor.Monitor, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.state[m.ID]
	if !ok {
		if s.Jitter <= 0 {
			return true
		}
		// Pretend the last run happened so that the first one lands
		// somewhere inside the jitter window.
		delay := time.Duration(rand.Int64N(int64(s.Jitter)))
		s.state[m.ID] = &monitorState{status: m.Status, lastRun: now.Add(delay - m.Interval)}
		return delay == 0
	}
	return !now.Before(st.lastRun.Add(m.Interval))
}

func (s *Scheduler) runCheck(ctx context.Context, m monitor.Monitor) {
	res := s.checker.Check(ctx, m)
	if err := s.store.RecordCheck(ctx, res); err != nil {
		s.logger.Error("record check", "monitor", m.ID, "err", err)
	}

	from, to := s.apply(m, res)
	if from != to {
		s.logger.Info("monitor status changed", "monitor", m.ID, "name", m.Name, "from", from, "to", to, "error", res.Error)
		if err := s.store.SetMonitorStatus(ctx, m.ID, to); err != nil {
			s.logger.Error("set status", "monitor", m.ID, "err", err)
		}
		if s.OnTransition != nil {
			s.OnTransition(ctx, Transition{Monitor: m, From: from, To: to, Result: res})
		}
	}
}

// apply folds a check result into the monitor state and returns the status
// before and after. A failing monitor is only marked down once it has used
// up its retries.
func (s *Scheduler) apply(m monitor.Monitor, res monitor.Result) (from, to monitor.Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.state[m.ID]
	if !ok {
		st = &monitorState{status: m.Status}
		s.state[m.ID] = st
	}
	st.lastRun = s.now()
	from = st.status

	if res.Up {
		st.failures = 0
		st.status = monitor.StatusUp
		return from, st.status
	}
	st.failures++
	// The first failure is not a retry: with Retries = 2 the monitor goes
	// down on the third consecutive failure.
	if st.failures > m.Retries {
		st.status = monitor.StatusDown
	}
	return from, st.status
}
