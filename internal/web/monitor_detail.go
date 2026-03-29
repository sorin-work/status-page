package web

import (
	"net/http"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/uptime"
)

const checksPerPage = 50

type monitorDetail struct {
	Monitor monitor.Monitor
	Uptime  uptime.Summary
	Checks  []monitor.Result
	Pager   pager
	Chart   latencyChart
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
	to := s.now().UTC()
	from := to.Add(-24 * time.Hour)
	series, err := s.store.LatencySeries(r.Context(), m.ID, from, 15*time.Minute)
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
	}})
}
