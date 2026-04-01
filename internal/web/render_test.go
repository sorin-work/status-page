package web

import (
	"testing"
	"time"
)

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
