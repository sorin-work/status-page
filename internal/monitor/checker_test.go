package monitor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func httpMonitor(url string) Monitor {
	return Monitor{ID: 7, Name: "test", Type: TypeHTTP, Target: url, Interval: time.Minute, Timeout: 2 * time.Second}
}

func TestHTTPCheckUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.UserAgent(), "status-page/") {
			t.Errorf("unexpected user agent %q", r.UserAgent())
		}
		_, _ = w.Write([]byte("all good"))
	}))
	defer srv.Close()

	res := NewChecker().Check(context.Background(), httpMonitor(srv.URL))
	if !res.Up || res.StatusCode != http.StatusOK || res.Error != "" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.MonitorID != 7 || res.CheckedAt.IsZero() || res.Latency <= 0 {
		t.Errorf("missing metadata: %+v", res)
	}
}

func TestHTTPCheckStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/created":
			w.WriteHeader(http.StatusCreated)
		case "/teapot":
			w.WriteHeader(http.StatusTeapot)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()

	cases := []struct {
		path     string
		expected int
		up       bool
	}{
		{"/created", 0, true},
		{"/down", 0, false},
		{"/teapot", 418, true},
		{"/created", 200, false},
	}
	for _, tc := range cases {
		m := httpMonitor(srv.URL + tc.path)
		m.ExpectedStatus = tc.expected
		res := NewChecker().Check(context.Background(), m)
		if res.Up != tc.up {
			t.Errorf("%s (expected %d): up = %v, err = %q", tc.path, tc.expected, res.Up, res.Error)
		}
	}
}

func TestHTTPCheckRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/old" {
			http.Redirect(w, r, "/new", http.StatusMovedPermanently)
			return
		}
		_, _ = w.Write([]byte("new page"))
	}))
	defer srv.Close()

	m := httpMonitor(srv.URL + "/old")
	m.ExpectedStatus = http.StatusOK
	m.Keyword = "new page"
	if res := NewChecker().Check(context.Background(), m); !res.Up {
		t.Errorf("redirect not followed: %+v", res)
	}
	m.NoRedirects = true
	m.ExpectedStatus = http.StatusMovedPermanently
	m.Keyword = ""
	if res := NewChecker().Check(context.Background(), m); !res.Up || res.StatusCode != http.StatusMovedPermanently {
		t.Errorf("redirect followed although disabled: %+v", res)
	}
}

func TestHTTPCheckKeyword(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	m := httpMonitor(srv.URL)
	m.Keyword = `"status":"ok"`
	if res := NewChecker().Check(context.Background(), m); !res.Up {
		t.Errorf("expected keyword match: %+v", res)
	}
	m.Keyword = "degraded"
	res := NewChecker().Check(context.Background(), m)
	if res.Up || !strings.Contains(res.Error, "keyword") {
		t.Errorf("expected keyword failure: %+v", res)
	}
}

func TestHTTPCheckTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	m := httpMonitor(srv.URL)
	m.Timeout = 50 * time.Millisecond
	res := NewChecker().Check(context.Background(), m)
	if res.Up || !strings.Contains(res.Error, "timed out") {
		t.Fatalf("expected timeout: %+v", res)
	}
}

func TestHTTPCheckConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()

	res := NewChecker().Check(context.Background(), httpMonitor(url))
	if res.Up || res.Error == "" {
		t.Fatalf("expected failure: %+v", res)
	}
}

func TestHTTPCheckKeywordBeyondLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat(".", MaxBodySize)))
		_, _ = w.Write([]byte("needle"))
	}))
	defer srv.Close()

	m := httpMonitor(srv.URL)
	m.Keyword = "needle"
	res := NewChecker().Check(context.Background(), m)
	if res.Up {
		t.Fatal("keyword after the body limit should not match")
	}
}
