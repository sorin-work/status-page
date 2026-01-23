// Package uptime turns check results into uptime percentages and the daily
// bars shown on the status page.
package uptime

import "fmt"

// Ratio counts successful checks out of all checks in a period.
type Ratio struct {
	Up    int
	Total int
}

// HasData reports whether any check was run in the period.
func (r Ratio) HasData() bool {
	return r.Total > 0
}

// Percent returns the uptime between 0 and 100. A period without checks
// counts as fully up.
func (r Ratio) Percent() float64 {
	if r.Total == 0 {
		return 100
	}
	return float64(r.Up) * 100 / float64(r.Total)
}

// Add combines two ratios.
func (r Ratio) Add(o Ratio) Ratio {
	return Ratio{Up: r.Up + o.Up, Total: r.Total + o.Total}
}

// String formats the ratio for display, e.g. "99.95%".
func (r Ratio) String() string {
	if !r.HasData() {
		return "no data"
	}
	return fmt.Sprintf("%.2f%%", r.Percent())
}
