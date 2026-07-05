package web

import (
	"testing"
	"time"
)

func TestDatetimeUsesLocation(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		t.Skip("no tz database")
	}
	s := &Server{loc: loc, now: time.Now}
	f := s.funcs()["datetime"].(func(time.Time) string)
	if got := f(time.Date(2026, 5, 12, 21, 0, 0, 0, time.UTC)); got != "2026-05-13 00:00 EEST" {
		t.Errorf("got %q", got)
	}
	if got := f(time.Time{}); got != "never" {
		t.Errorf("zero time = %q", got)
	}
}

func TestNl2br(t *testing.T) {
	cases := map[string]string{
		"plain":                        "plain",
		"one\ntwo":                     "one<br>two",
		"windows\r\nline":              "windows<br>line",
		"<script>alert(1)</script>":    "&lt;script&gt;alert(1)&lt;/script&gt;",
		`<img src=x onerror="evil()">`: "&lt;img src=x onerror=&#34;evil()&#34;&gt;",
	}
	for in, want := range cases {
		if got := string(nl2br(in)); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

func TestFormatAgo(t *testing.T) {
	cases := map[time.Duration]string{
		time.Second:      "just now",
		42 * time.Second: "42s ago",
		5 * time.Minute:  "5m ago",
		26 * time.Hour:   "26h ago",
		72 * time.Hour:   "3d ago",
	}
	for d, want := range cases {
		if got := formatAgo(d); got != want {
			t.Errorf("%v: got %q, want %q", d, got, want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{
		0:                                   "0s",
		250 * time.Millisecond:              "250ms",
		30 * time.Second:                    "30s",
		5 * time.Minute:                     "5m",
		2 * time.Hour:                       "2h",
		90 * time.Minute:                    "1h30m0s",
		time.Minute + 1500*time.Millisecond: "1m1s",
	}
	for d, want := range cases {
		if got := formatDuration(d); got != want {
			t.Errorf("%v: got %q, want %q", d, got, want)
		}
	}
}
