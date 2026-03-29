package web

import (
	"fmt"
	"strings"
	"time"

	"github.com/sorin-work/status-page/internal/store"
)

// Chart dimensions in SVG user units.
const (
	chartWidth  = 800
	chartHeight = 160
	chartPad    = 4
)

// chartMark is a red tick for a bucket with failed checks.
type chartMark struct {
	X     float64
	Title string
}

type latencyChart struct {
	Width, Height int
	Points        string
	Marks         []chartMark
	MaxLabel      string
	AvgLabel      string
	From, To      time.Time
	Empty         bool
}

// buildLatencyChart scales the series into an SVG polyline covering the
// window [from, to].
func buildLatencyChart(points []store.LatencyPoint, from, to time.Time) latencyChart {
	c := latencyChart{Width: chartWidth, Height: chartHeight, From: from, To: to}
	var (
		peak      time.Duration
		sum       time.Duration
		withValue int
	)
	for _, p := range points {
		if p.Avg > peak {
			peak = p.Avg
		}
		if p.Avg > 0 {
			sum += p.Avg
			withValue++
		}
	}
	if withValue == 0 {
		c.Empty = true
	} else {
		c.AvgLabel = formatDuration(sum / time.Duration(withValue))
	}
	if peak <= 0 {
		peak = time.Millisecond
	}
	c.MaxLabel = formatDuration(peak)

	span := to.Sub(from)
	if span <= 0 {
		return c
	}
	x := func(t time.Time) float64 {
		return float64(t.Sub(from)) / float64(span) * chartWidth
	}
	var b strings.Builder
	for _, p := range points {
		if p.Failures > 0 {
			c.Marks = append(c.Marks, chartMark{X: x(p.Start), Title: fmt.Sprintf("%d failed checks", p.Failures)})
		}
		if p.Avg <= 0 {
			continue
		}
		y := chartHeight - chartPad - float64(p.Avg)/float64(peak)*(chartHeight-2*chartPad)
		fmt.Fprintf(&b, "%.1f,%.1f ", x(p.Start), y)
	}
	c.Points = strings.TrimSpace(b.String())
	return c
}
