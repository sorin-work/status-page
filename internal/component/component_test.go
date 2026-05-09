package component

import (
	"testing"

	"github.com/sorin-work/status-page/internal/incident"
	"github.com/sorin-work/status-page/internal/monitor"
)

func mons(statuses ...monitor.Status) []monitor.Monitor {
	out := make([]monitor.Monitor, len(statuses))
	for i, s := range statuses {
		out[i] = monitor.Monitor{Status: s}
	}
	return out
}

func TestLevelFor(t *testing.T) {
	up, down, unknown := monitor.StatusUp, monitor.StatusDown, monitor.StatusUnknown
	cases := []struct {
		name string
		in   []monitor.Monitor
		want Level
	}{
		{"no monitors", nil, NoData},
		{"never checked", mons(unknown, unknown), NoData},
		{"all up", mons(up, up), Operational},
		{"some down", mons(up, down, unknown), PartialOutage},
		{"all down", mons(down, down), MajorOutage},
		{"paused are ignored", append(mons(up), monitor.Monitor{Status: down, Paused: true}), Operational},
	}
	for _, tc := range cases {
		if got := LevelFor(tc.in); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestOverall(t *testing.T) {
	if got := Overall(nil); got != NoData {
		t.Errorf("empty = %s", got)
	}
	if got := Overall([]Level{Operational, NoData, Operational}); got != Operational {
		t.Errorf("got %s", got)
	}
	if got := Overall([]Level{Operational, Maintenance}); got != Maintenance {
		t.Errorf("got %s", got)
	}
	if got := Overall([]Level{Maintenance, PartialOutage, Operational}); got != PartialOutage {
		t.Errorf("got %s", got)
	}
	if got := Overall([]Level{PartialOutage, MajorOutage}); got != MajorOutage {
		t.Errorf("got %s", got)
	}
}

func TestForIncident(t *testing.T) {
	cases := map[incident.Impact]Level{
		incident.ImpactNone:     Operational,
		incident.ImpactMinor:    PartialOutage,
		incident.ImpactMajor:    MajorOutage,
		incident.ImpactCritical: MajorOutage,
	}
	for impact, want := range cases {
		if got := ForIncident(impact); got != want {
			t.Errorf("%s: got %s, want %s", impact, got, want)
		}
	}
}

func TestLabelsAndHeadlines(t *testing.T) {
	for _, l := range []Level{Operational, Maintenance, PartialOutage, MajorOutage, NoData} {
		if l.Label() == "" || Headline(l) == "" {
			t.Errorf("%s has no text", l)
		}
	}
	if Headline(Operational) != "All systems operational" {
		t.Error("unexpected headline")
	}
}
