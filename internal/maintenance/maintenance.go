// Package maintenance defines planned maintenance windows.
package maintenance

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxDuration is the longest window that can be scheduled.
const MaxDuration = 7 * 24 * time.Hour

// Window is a planned period of downtime. During a window the affected
// monitors do not open incidents and the status page shows maintenance
// instead of an outage.
type Window struct {
	ID          int64
	Title       string
	Description string
	// ComponentID limits the window to one component; zero means every
	// component.
	ComponentID int64
	StartsAt    time.Time
	EndsAt      time.Time
	CreatedAt   time.Time
}

// Active reports whether the window is in progress at t.
func (w Window) Active(t time.Time) bool {
	return !t.Before(w.StartsAt) && t.Before(w.EndsAt)
}

// Upcoming reports whether the window starts after t.
func (w Window) Upcoming(t time.Time) bool {
	return t.Before(w.StartsAt)
}

// Covers reports whether the window applies to a component.
func (w Window) Covers(componentID int64) bool {
	return w.ComponentID == 0 || w.ComponentID == componentID
}

// ValidationError maps field names to problems.
type ValidationError map[string]string

func (v ValidationError) Error() string {
	parts := make([]string, 0, len(v))
	for _, k := range []string{"title", "description", "starts_at", "ends_at"} {
		if msg, ok := v[k]; ok {
			parts = append(parts, k+": "+msg)
		}
	}
	return "invalid maintenance window: " + strings.Join(parts, "; ")
}

// Validate checks a window before it is saved. now is used to reject
// windows that are already over.
func (w Window) Validate(now time.Time) error {
	errs := ValidationError{}
	title := strings.TrimSpace(w.Title)
	switch {
	case title == "":
		errs["title"] = "is required"
	case utf8.RuneCountInString(title) > 200:
		errs["title"] = "must be at most 200 characters"
	}
	if utf8.RuneCountInString(w.Description) > 2000 {
		errs["description"] = "must be at most 2000 characters"
	}
	switch {
	case w.StartsAt.IsZero():
		errs["starts_at"] = "is required"
	case w.EndsAt.IsZero():
		errs["ends_at"] = "is required"
	case !w.EndsAt.After(w.StartsAt):
		errs["ends_at"] = "must be after the start"
	case w.EndsAt.Sub(w.StartsAt) > MaxDuration:
		errs["ends_at"] = fmt.Sprintf("window can be at most %d days long", int(MaxDuration.Hours()/24))
	case !w.EndsAt.After(now):
		errs["ends_at"] = "is in the past"
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}
