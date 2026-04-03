// Package incident defines incidents and their timeline.
package incident

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Status is the phase an incident is in.
type Status string

// Incident phases, in the order they usually happen.
const (
	StatusInvestigating Status = "investigating"
	StatusIdentified    Status = "identified"
	StatusMonitoring    Status = "monitoring"
	StatusResolved      Status = "resolved"
)

// Statuses lists every status in display order.
var Statuses = []Status{StatusInvestigating, StatusIdentified, StatusMonitoring, StatusResolved}

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	for _, v := range Statuses {
		if s == v {
			return true
		}
	}
	return false
}

// Impact is how badly an incident affects users.
type Impact string

// Impact levels, from harmless to severe.
const (
	ImpactNone     Impact = "none"
	ImpactMinor    Impact = "minor"
	ImpactMajor    Impact = "major"
	ImpactCritical Impact = "critical"
)

// Impacts lists every impact in display order.
var Impacts = []Impact{ImpactNone, ImpactMinor, ImpactMajor, ImpactCritical}

// Valid reports whether i is a known impact.
func (i Impact) Valid() bool {
	for _, v := range Impacts {
		if i == v {
			return true
		}
	}
	return false
}

// Incident is a problem shown on the status page.
type Incident struct {
	ID     int64
	Title  string
	Status Status
	Impact Impact
	// MonitorID is set for incidents opened automatically by a monitor.
	MonitorID  int64
	Automatic  bool
	StartedAt  time.Time
	ResolvedAt time.Time
	UpdatedAt  time.Time
	Updates    []Update
}

// Resolved reports whether the incident is over.
func (i Incident) Resolved() bool {
	return i.Status == StatusResolved
}

// Duration is how long the incident lasted, or has lasted so far.
func (i Incident) Duration(now time.Time) time.Duration {
	end := now
	if !i.ResolvedAt.IsZero() {
		end = i.ResolvedAt
	}
	return end.Sub(i.StartedAt)
}

// Update is one entry in the incident timeline.
type Update struct {
	ID         int64
	IncidentID int64
	Status     Status
	Message    string
	CreatedAt  time.Time
}

// Limits for user input.
const (
	MaxTitleLength   = 200
	MaxMessageLength = 5000
)

// ValidationError maps field names to problems.
type ValidationError map[string]string

func (v ValidationError) Error() string {
	parts := make([]string, 0, len(v))
	for _, k := range []string{"title", "status", "impact", "message"} {
		if msg, ok := v[k]; ok {
			parts = append(parts, k+": "+msg)
		}
	}
	return "invalid incident: " + strings.Join(parts, "; ")
}

// ValidateNew checks the fields used to open an incident by hand.
func ValidateNew(title string, status Status, impact Impact, message string) error {
	errs := ValidationError{}
	title = strings.TrimSpace(title)
	switch {
	case title == "":
		errs["title"] = "is required"
	case utf8.RuneCountInString(title) > MaxTitleLength:
		errs["title"] = fmt.Sprintf("must be at most %d characters", MaxTitleLength)
	}
	if !status.Valid() {
		errs["status"] = "is not a valid status"
	}
	if !impact.Valid() {
		errs["impact"] = "is not a valid impact"
	}
	addMessageError(errs, message)
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// ValidateUpdate checks a timeline update.
func ValidateUpdate(status Status, message string) error {
	errs := ValidationError{}
	if !status.Valid() {
		errs["status"] = "is not a valid status"
	}
	addMessageError(errs, message)
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func addMessageError(errs ValidationError, message string) {
	message = strings.TrimSpace(message)
	switch {
	case message == "":
		errs["message"] = "is required"
	case utf8.RuneCountInString(message) > MaxMessageLength:
		errs["message"] = fmt.Sprintf("must be at most %d characters", MaxMessageLength)
	}
}
