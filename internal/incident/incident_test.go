package incident

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStatusAndImpactValid(t *testing.T) {
	for _, s := range Statuses {
		if !s.Valid() {
			t.Errorf("%s should be valid", s)
		}
	}
	if Status("closed").Valid() || Impact("huge").Valid() {
		t.Error("unknown values accepted")
	}
	for _, i := range Impacts {
		if !i.Valid() {
			t.Errorf("%s should be valid", i)
		}
	}
}

func TestValidateNew(t *testing.T) {
	if err := ValidateNew("Login errors", StatusInvestigating, ImpactMinor, "We are looking into it."); err != nil {
		t.Fatalf("valid incident rejected: %v", err)
	}
	cases := []struct {
		name    string
		field   string
		title   string
		status  Status
		impact  Impact
		message string
	}{
		{"no title", "title", "  ", StatusInvestigating, ImpactMinor, "x"},
		{"long title", "title", strings.Repeat("t", 201), StatusInvestigating, ImpactMinor, "x"},
		{"bad status", "status", "t", "broken", ImpactMinor, "x"},
		{"bad impact", "impact", "t", StatusInvestigating, "", "x"},
		{"no message", "message", "t", StatusInvestigating, ImpactMinor, "\n"},
		{"long message", "message", "t", StatusInvestigating, ImpactMinor, strings.Repeat("m", 5001)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateNew(tc.title, tc.status, tc.impact, tc.message)
			var verr ValidationError
			if !errors.As(err, &verr) || verr[tc.field] == "" {
				t.Fatalf("expected an error on %s, got %v", tc.field, err)
			}
		})
	}
}

func TestValidateUpdate(t *testing.T) {
	if err := ValidateUpdate(StatusResolved, "Fixed."); err != nil {
		t.Fatal(err)
	}
	err := ValidateUpdate("", "")
	var verr ValidationError
	if !errors.As(err, &verr) || len(verr) != 2 {
		t.Fatalf("got %v", err)
	}
	if verr.Error() != "invalid incident: status: is not a valid status; message: is required" {
		t.Errorf("message = %q", verr.Error())
	}
}

func TestDuration(t *testing.T) {
	start := time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)
	open := Incident{StartedAt: start}
	if d := open.Duration(start.Add(time.Hour)); d != time.Hour {
		t.Errorf("open incident duration = %v", d)
	}
	closed := Incident{StartedAt: start, ResolvedAt: start.Add(20 * time.Minute), Status: StatusResolved}
	if d := closed.Duration(start.Add(5 * time.Hour)); d != 20*time.Minute || !closed.Resolved() {
		t.Errorf("resolved incident duration = %v", d)
	}
}
