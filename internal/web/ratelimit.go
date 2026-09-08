package web

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// loginLimiter counts failed logins per client address and blocks the
// address for a while after too many.
type loginLimiter struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	failures map[string][]time.Time
	now      func() time.Time
}

func newLoginLimiter(maxFailures int, window time.Duration, now func() time.Time) *loginLimiter {
	return &loginLimiter{max: maxFailures, window: window, failures: map[string][]time.Time{}, now: now}
}

// recent drops failures outside the window and returns what is left.
func (l *loginLimiter) recent(key string) []time.Time {
	cutoff := l.now().Add(-l.window)
	kept := l.failures[key][:0]
	for _, t := range l.failures[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.failures, key)
		return nil
	}
	l.failures[key] = kept
	return kept
}

// Blocked reports whether key has used up its attempts, and for how long.
func (l *loginLimiter) Blocked(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	list := l.recent(key)
	if len(list) < l.max {
		return false, 0
	}
	return true, list[0].Add(l.window).Sub(l.now())
}

// Fail records a failed attempt.
func (l *loginLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failures[key] = append(l.recent(key), l.now())
}

// Reset forgets the failures of key after a successful login.
func (l *loginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}

// clientIP is the address used for rate limiting and logs. The proxy
// header is only honoured with TRUST_PROXY, otherwise anyone could pick a
// new address for every login attempt.
func (s *Server) clientIP(r *http.Request) string {
	if s.trustProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			// The last entry was added by our proxy.
			parts := strings.Split(fwd, ",")
			if ip := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
