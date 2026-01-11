// Package scheduler runs monitor checks on their configured intervals.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
)

// Store is the persistence the scheduler needs.
type Store interface {
	ListActiveMonitors(ctx context.Context) ([]monitor.Monitor, error)
	RecordCheck(ctx context.Context, r monitor.Result) error
	SetMonitorStatus(ctx context.Context, id int64, status monitor.Status) error
}

// Scheduler decides which monitors are due and runs their checks.
type Scheduler struct {
	store   Store
	checker monitor.Checker
	logger  *slog.Logger
	now     func() time.Time

	// Tick is how often the scheduler looks for due monitors.
	Tick time.Duration

	lastRun map[int64]time.Time
}

// New returns a scheduler that uses store and checker.
func New(store Store, checker monitor.Checker, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		store:   store,
		checker: checker,
		logger:  logger,
		now:     time.Now,
		Tick:    time.Second,
		lastRun: map[int64]time.Time{},
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
			return
		case <-t.C:
			s.RunDue(ctx)
		}
	}
}

// RunDue runs the checks of every monitor whose interval has elapsed.
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
		s.runCheck(ctx, m)
	}
}

func (s *Scheduler) isDue(m monitor.Monitor, now time.Time) bool {
	last, ok := s.lastRun[m.ID]
	return !ok || !now.Before(last.Add(m.Interval))
}

func (s *Scheduler) runCheck(ctx context.Context, m monitor.Monitor) {
	res := s.checker.Check(ctx, m)
	s.lastRun[m.ID] = s.now()
	if err := s.store.RecordCheck(ctx, res); err != nil {
		s.logger.Error("record check", "monitor", m.ID, "err", err)
	}
	status := monitor.StatusDown
	if res.Up {
		status = monitor.StatusUp
	}
	if status != m.Status {
		if err := s.store.SetMonitorStatus(ctx, m.ID, status); err != nil {
			s.logger.Error("set status", "monitor", m.ID, "err", err)
		}
	}
}
