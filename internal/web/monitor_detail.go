package web

import (
	"net/http"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/uptime"
)

const checksPerPage = 50

// chartRange is a selectable period for the response time chart.
type chartRange struct {
	Key    string
	Label  string
	Span   time.Duration
	Bucket time.Duration
}

var chartRanges = []chartRange{
	{Key: "24h", Label: "24 hours", Span: 24 * time.Hour, Bucket: 15 * time.Minute},
	{Key: "7d", Label: "7 days", Span: 7 * 24 * time.Hour, Bucket: 2 * time.Hour},
}

func pickChartRange(key string) chartRange {
	for _, cr := range chartRanges {
		if cr.Key == key {
			return cr
		}
	}
	return chartRanges[0]
}

type monitorDetail struct {
	Monitor monitor.Monitor
	Uptime  uptime.Summary
	Checks  []monitor.Result
	Pager   pager
	Chart   latencyChart
	Range   chartRange
	Ranges  []chartRange
}

func (s *Server) handleMonitorDetail(w http.ResponseWriter, r *http.Request) {
	m, ok := s.loadMonitor(w, r)
	if !ok {
		return
	}
	sum, err := uptime.Summarize(r.Context(), s.store, m.ID, s.now())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	cr := pickChartRange(r.URL.Query().Get("range"))
	to := s.now().UTC()
	from := to.Add(-cr.Span)
	series, err := s.store.LatencySeries(r.Context(), m.ID, from, cr.Bucket)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	total, err := s.store.CountChecks(r.Context(), m.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	pg := newPager(r, total, checksPerPage)
	checks, err := s.store.ListChecks(r.Context(), m.ID, pg.PerPage, pg.Offset())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "monitor_detail", page{Title: m.Name, Nav: "monitors", Data: monitorDetail{
		Monitor: m,
		Uptime:  sum,
		Checks:  checks,
		Pager:   pg,
		Chart:   buildLatencyChart(series, from, to),
		Range:   cr,
		Ranges:  chartRanges,
	}})
}
