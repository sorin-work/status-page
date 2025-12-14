// Package monitor defines monitors and the checks that are run against them.
package monitor

import "time"

// Type is the kind of check a monitor performs.
type Type string

// Supported monitor types.
const (
	TypeHTTP Type = "http"
	TypeTCP  Type = "tcp"
)

// Valid reports whether t is a known monitor type.
func (t Type) Valid() bool {
	return t == TypeHTTP || t == TypeTCP
}

// Status is the last known state of a monitor.
type Status string

// Monitor states.
const (
	StatusUnknown Status = "unknown"
	StatusUp      Status = "up"
	StatusDown    Status = "down"
)

// Monitor describes something that is checked on a fixed interval.
type Monitor struct {
	ID   int64
	Name string
	Type Type
	// Target is a URL for HTTP monitors and host:port for TCP monitors.
	Target   string
	Interval time.Duration
	Timeout  time.Duration
	// ExpectedStatus is the HTTP status code a check must return. Zero
	// accepts any 2xx or 3xx response.
	ExpectedStatus int
	// Keyword, when set, must appear in the HTTP response body.
	Keyword string
	// Retries is how many extra failed checks are tolerated before the
	// monitor is marked down.
	Retries   int
	Paused    bool
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Result is the outcome of a single check.
type Result struct {
	MonitorID  int64
	CheckedAt  time.Time
	Up         bool
	Latency    time.Duration
	StatusCode int
	Error      string
}
