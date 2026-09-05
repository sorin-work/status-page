package monitor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// MaxBodySize is how much of an HTTP response is read when looking for
// the keyword. Anything after it is ignored.
const MaxBodySize = 1 << 20

// Checker runs a single check for a monitor.
type Checker interface {
	Check(ctx context.Context, m Monitor) Result
}

// NetChecker performs real HTTP and TCP checks.
type NetChecker struct {
	Client    *http.Client
	Dialer    *net.Dialer
	UserAgent string
	Now       func() time.Time
}

// NewChecker returns a NetChecker with sensible defaults.
func NewChecker() *NetChecker {
	return &NetChecker{
		Client:    &http.Client{},
		Dialer:    &net.Dialer{},
		UserAgent: "status-page/0.1",
		Now:       time.Now,
	}
}

// Check runs the check that matches the monitor type.
func (c *NetChecker) Check(ctx context.Context, m Monitor) Result {
	switch m.Type {
	case TypeHTTP:
		return c.checkHTTP(ctx, m)
	case TypeTCP:
		return c.checkTCP(ctx, m)
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

	client := c.Client
	if m.NoRedirects {
		noFollow := *c.Client
		noFollow.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		client = &noFollow
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		res.Latency = time.Since(start)
		res.Error = describeError(err, m.Timeout)
		return res
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBodySize))
	res.Latency = time.Since(start)
	if err != nil {
		res.Error = "reading response: " + err.Error()
		return res
	}
	res.StatusCode = resp.StatusCode

	if m.ExpectedStatus != 0 {
		if resp.StatusCode != m.ExpectedStatus {
			res.Error = fmt.Sprintf("expected status %d, got %d", m.ExpectedStatus, resp.StatusCode)
			return res
		}
	} else if resp.StatusCode < 200 || resp.StatusCode > 399 {
		res.Error = fmt.Sprintf("unexpected status %d", resp.StatusCode)
		return res
	}
	if m.Keyword != "" && !bytes.Contains(body, []byte(m.Keyword)) {
		res.Error = fmt.Sprintf("keyword %q not found in response", m.Keyword)
		return res
	}
	res.Up = true
	return res
}

func (c *NetChecker) checkTCP(ctx context.Context, m Monitor) Result {
	res := Result{MonitorID: m.ID, CheckedAt: c.Now().UTC()}
	ctx, cancel := context.WithTimeout(ctx, m.Timeout)
	defer cancel()

	start := time.Now()
	conn, err := c.Dialer.DialContext(ctx, "tcp", m.Target)
	res.Latency = time.Since(start)
	if err != nil {
		res.Error = describeError(err, m.Timeout)
		return res
	}
	_ = conn.Close()
	res.Up = true
	return res
}

// describeError turns a transport error into a short message for the
// check history.
func describeError(err error, timeout time.Duration) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Sprintf("timed out after %s", timeout)
	}
	return err.Error()
}
