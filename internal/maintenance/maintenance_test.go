package maintenance

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 5, 14, 20, 0, 0, 0, time.UTC)

func window(start, end time.Duration) Window {
	return Window{Title: "Database upgrade", StartsAt: now.Add(start), EndsAt: now.Add(end)}
}

func TestActiveAndUpcoming(t *testing.T) {
	w := window(time.Hour, 2*time.Hour)
	if w.Active(now) || !w.Upcoming(now) {
		t.Error("future window")
	}
	if !w.Active(now.Add(time.Hour)) || !w.Active(now.Add(119*time.Minute)) {
		t.Error("window should be active inside its bounds")
	}
	if w.Active(now.Add(2*time.Hour)) || w.Upcoming(now.Add(3*time.Hour)) {
		t.Error("finished window")
	}
}

func TestCovers(t *testing.T) {
	all := Window{}
	one := Window{ComponentID: 3}
	if !all.Covers(3) || !all.Covers(9) || !one.Covers(3) || one.Covers(9) {
		t.Error("unexpected coverage")
	}
}

func TestValidate(t *testing.T) {
	if err := window(time.Hour, 3*time.Hour).Validate(now); err != nil {
		t.Fatalf("valid window rejected: %v", err)
	}
	// Already running windows can still be added.
	if err := window(-time.Hour, time.Hour).Validate(now); err != nil {
		t.Fatalf("running window rejected: %v", err)
	}
	cases := []struct {
		name  string
		field string
		w     Window
	}{
		{"no title", "title", Window{StartsAt: now, EndsAt: now.Add(time.Hour)}},
		{"long title", "title", Window{Title: strings.Repeat("x", 201), StartsAt: now, EndsAt: now.Add(time.Hour)}},
		{"long description", "description", Window{Title: "x", Description: strings.Repeat("d", 2001), StartsAt: now, EndsAt: now.Add(time.Hour)}},
		{"no start", "starts_at", Window{Title: "x", EndsAt: now.Add(time.Hour)}},
		{"no end", "ends_at", Window{Title: "x", StartsAt: now}},
		{"end before start", "ends_at", window(2*time.Hour, time.Hour)},
		{"too long", "ends_at", window(0, 8*24*time.Hour)},
		{"in the past", "ends_at", window(-3*time.Hour, -time.Hour)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var verr ValidationError
			if err := tc.w.Validate(now); !errors.As(err, &verr) || verr[tc.field] == "" {
				t.Fatalf("expected error on %s, got %v", tc.field, err)
			}
		})
	}
}
