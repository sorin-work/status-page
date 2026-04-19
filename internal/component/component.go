// Package component works out the public status of status page components
// from the monitors behind them.
package component

import "github.com/sorin-work/status-page/internal/monitor"

// Level is the public state of a component or of the whole page.
type Level string

// Levels, from best to worst. Maintenance is reported separately because
// it is planned.
const (
	Operational   Level = "operational"
	Maintenance   Level = "maintenance"
	PartialOutage Level = "partial_outage"
	MajorOutage   Level = "major_outage"
	NoData        Level = "nodata"
)

var severity = map[Level]int{
	NoData:        0,
	Operational:   1,
	Maintenance:   2,
	PartialOutage: 3,
	MajorOutage:   4,
}

// Worse reports whether a is more severe than b.
func Worse(a, b Level) bool {
	return severity[a] > severity[b]
}

// Label is the text shown next to a component.
func (l Level) Label() string {
	switch l {
	case Operational:
		return "Operational"
	case Maintenance:
		return "Under maintenance"
	case PartialOutage:
		return "Partial outage"
	case MajorOutage:
		return "Major outage"
	default:
		return "No data"
	}
}

// LevelFor derives the level of a component from its monitors. Paused
// monitors and monitors that have not been checked yet are ignored.
func LevelFor(monitors []monitor.Monitor) Level {
	var known, down int
	for _, m := range monitors {
		if m.Paused {
			continue
		}
		switch m.Status {
		case monitor.StatusUp:
			known++
		case monitor.StatusDown:
			known++
			down++
		}
	}
	switch {
	case known == 0:
		return NoData
	case down == 0:
		return Operational
	case down == known:
		return MajorOutage
	default:
		return PartialOutage
	}
}

// Overall summarizes the levels of all components for the banner at the
// top of the status page.
func Overall(levels []Level) Level {
	overall := NoData
	for _, l := range levels {
		if Worse(l, overall) {
			overall = l
		}
	}
	return overall
}

// Headline is the banner text for an overall level.
func Headline(l Level) string {
	switch l {
	case Operational:
		return "All systems operational"
	case Maintenance:
		return "Scheduled maintenance in progress"
	case PartialOutage:
		return "Some systems are experiencing issues"
	case MajorOutage:
		return "Major outage"
	default:
		return "Status unknown"
	}
}
