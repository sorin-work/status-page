package web

import (
	"testing"
	"time"
)

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
