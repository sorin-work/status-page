package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestLoginLimiter(t *testing.T) {
	now := time.Date(2026, 7, 19, 19, 0, 0, 0, time.UTC)
	l := newLoginLimiter(3, 10*time.Minute, func() time.Time { return now })
	for range 2 {
		l.Fail("1.2.3.4")
	}
	if blocked, _ := l.Blocked("1.2.3.4"); blocked {
		t.Fatal("blocked before the limit")
	}
	l.Fail("1.2.3.4")
	blocked, wait := l.Blocked("1.2.3.4")
	if !blocked || wait != 10*time.Minute {
		t.Fatalf("blocked=%v wait=%v", blocked, wait)
	}
	if blocked, _ := l.Blocked("5.6.7.8"); blocked {
		t.Error("other addresses are blocked too")
	}
	now = now.Add(11 * time.Minute)
	if blocked, _ := l.Blocked("1.2.3.4"); blocked {
		t.Error("still blocked after the window")
	}
	l.Fail("1.2.3.4")
	l.Reset("1.2.3.4")
	if len(l.failures) != 0 {
		t.Error("reset kept failures")
	}
}

func TestLoginIsRateLimited(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	wrong := url.Values{"username": {"admin"}, "password": {"guess"}}
	for i := range 5 {
		if rec := postForm(t, srv, "/admin/login", wrong); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i+1, rec.Code)
		}
	}
	// Even the right password is refused while blocked.
	rec := postForm(t, srv, "/admin/login", url.Values{"username": {"admin"}, "password": {"demo-password"}})
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestClientIP(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:51234"
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 198.51.100.7")
	if got := srv.clientIP(req); got != "10.0.0.2" {
		t.Errorf("without TRUST_PROXY: %q", got)
	}
	srv.trustProxy = true
	if got := srv.clientIP(req); got != "198.51.100.7" {
		t.Errorf("with TRUST_PROXY: %q", got)
	}
	req.Header.Set("X-Forwarded-For", "garbage")
	if got := srv.clientIP(req); got != "10.0.0.2" {
		t.Errorf("invalid header: %q", got)
	}
}
