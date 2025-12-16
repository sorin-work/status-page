package monitor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Checker runs a single check for a monitor.
type Checker interface {
	Check(ctx context.Context, m Monitor) Result
}

// NetChecker performs real HTTP and TCP checks.
type NetChecker struct {
	Client    *http.Client
	UserAgent string
	Now       func() time.Time
}

// NewChecker returns a NetChecker with sensible defaults.
func NewChecker() *NetChecker {
	return &NetChecker{
		Client:    &http.Client{},
		UserAgent: "status-page/0.1",
		Now:       time.Now,
	}
}

// Check runs the check that matches the monitor type.
func (c *NetChecker) Check(ctx context.Context, m Monitor) Result {
	switch m.Type {
	case TypeHTTP:
		return c.checkHTTP(ctx, m)
	default:
		return Result{MonitorID: m.ID, CheckedAt: c.Now().UTC(), Error: fmt.Sprintf("unsupported monitor type %q", m.Type)}
	}
}

func (c *NetChecker) checkHTTP(ctx context.Context, m Monitor) Result {
	res := Result{MonitorID: m.ID, CheckedAt: c.Now().UTC()}
	ctx, cancel := context.WithTimeout(ctx, m.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.Target, nil)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	req.Header.Set("User-Agent", c.UserAgent)

	start := time.Now()
	resp, err := c.Client.Do(req)
	if err != nil {
		res.Latency = time.Since(start)
		if errors.Is(err, context.DeadlineExceeded) {
			res.Error = fmt.Sprintf("timed out after %s", m.Timeout)
		} else {
			res.Error = err.Error()
		}
		return res
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	res.Latency = time.Since(start)
	res.StatusCode = resp.StatusCode

	if resp.StatusCode < 200 || resp.StatusCode > 399 {
		res.Error = fmt.Sprintf("unexpected status %d", resp.StatusCode)
		return res
	}
	res.Up = true
	return res
}
