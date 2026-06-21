package monitor

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func validHTTP() Monitor {
	return Monitor{
		Name:     "Website",
		Type:     TypeHTTP,
		Target:   "https://example.com/health",
		Interval: time.Minute,
		Timeout:  10 * time.Second,
	}
}

func validTCP() Monitor {
	return Monitor{
		Name:     "Database",
		Type:     TypeTCP,
		Target:   "db.internal:5432",
		Interval: time.Minute,
		Timeout:  5 * time.Second,
	}
}

func TestValidateAcceptsValidMonitors(t *testing.T) {
	for _, m := range []Monitor{validHTTP(), validTCP()} {
		if err := m.Validate(); err != nil {
			t.Errorf("%s: unexpected error: %v", m.Name, err)
		}
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name  string
		field string
		edit  func(*Monitor)
	}{
		{"empty name", "name", func(m *Monitor) { m.Name = "" }},
		{"long name", "name", func(m *Monitor) { m.Name = strings.Repeat("a", 101) }},
		{"unknown type", "type", func(m *Monitor) { m.Type = "icmp" }},
		{"empty target", "target", func(m *Monitor) { m.Target = "" }},
		{"ftp scheme", "target", func(m *Monitor) { m.Target = "ftp://example.com" }},
		{"no scheme", "target", func(m *Monitor) { m.Target = "example.com" }},
		{"no host", "target", func(m *Monitor) { m.Target = "https:///path" }},
		{"interval too short", "interval", func(m *Monitor) { m.Interval = 5 * time.Second }},
		{"interval too long", "interval", func(m *Monitor) { m.Interval = 48 * time.Hour }},
		{"timeout too short", "timeout", func(m *Monitor) { m.Timeout = 100 * time.Millisecond }},
		{"timeout too long", "timeout", func(m *Monitor) { m.Timeout = 2 * time.Minute; m.Interval = time.Hour }},
		{"timeout not below interval", "timeout", func(m *Monitor) { m.Interval = 30 * time.Second; m.Timeout = 30 * time.Second }},
		{"negative retries", "retries", func(m *Monitor) { m.Retries = -1 }},
		{"too many retries", "retries", func(m *Monitor) { m.Retries = 11 }},
		{"bad status code", "expected_status", func(m *Monitor) { m.ExpectedStatus = 42 }},
		{"huge keyword", "keyword", func(m *Monitor) { m.Keyword = strings.Repeat("k", 201) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := validHTTP()
			tc.edit(&m)
			assertFieldError(t, m.Validate(), tc.field)
		})
	}
}

func TestValidateTCPRejects(t *testing.T) {
	cases := []struct {
		name  string
		field string
		edit  func(*Monitor)
	}{
		{"scheme", "target", func(m *Monitor) { m.Target = "tcp://db:5432" }},
		{"path", "target", func(m *Monitor) { m.Target = "db:5432/x" }},
		{"expected status", "expected_status", func(m *Monitor) { m.ExpectedStatus = 200 }},
		// Without a port every check failed with "missing port in address".
		{"no port", "target", func(m *Monitor) { m.Target = "db.internal" }},
		{"empty port", "target", func(m *Monitor) { m.Target = "db.internal:" }},
		{"no host", "target", func(m *Monitor) { m.Target = ":5432" }},
		{"port out of range", "target", func(m *Monitor) { m.Target = "db.internal:70000" }},
		{"port zero", "target", func(m *Monitor) { m.Target = "db.internal:0" }},
		{"named port", "target", func(m *Monitor) { m.Target = "db.internal:postgres" }},
		{"keyword", "keyword", func(m *Monitor) { m.Keyword = "ok" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := validTCP()
			tc.edit(&m)
			assertFieldError(t, m.Validate(), tc.field)
		})
	}
}

func TestValidateTCPAcceptsIPv6(t *testing.T) {
	m := validTCP()
	m.Target = "[2001:db8::1]:5432"
	if err := m.Validate(); err != nil {
		t.Errorf("IPv6 target rejected: %v", err)
	}
}

func TestNormalize(t *testing.T) {
	m := Monitor{Name: "  API ", Type: " HTTP", Target: " https://x.test ", Keyword: " ok "}
	m.Normalize()
	if m.Name != "API" || m.Type != TypeHTTP || m.Target != "https://x.test" || m.Keyword != "ok" {
		t.Errorf("unexpected result: %+v", m)
	}
}

func TestValidationErrorMessageIsSorted(t *testing.T) {
	err := ValidationError{"timeout": "bad", "name": "missing"}
	if got, want := err.Error(), "invalid monitor: name: missing; timeout: bad"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func assertFieldError(t *testing.T, err error, field string) {
	t.Helper()
	var verr ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if _, ok := verr[field]; !ok {
		t.Fatalf("expected error on %q, got %v", field, verr)
	}
}
