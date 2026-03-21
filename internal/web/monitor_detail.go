package web

import (
	"net/http"

	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/uptime"
)

const checksPerPage = 50

type monitorDetail struct {
	Monitor monitor.Monitor
	Uptime  uptime.Summary
	Checks  []monitor.Result
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
	checks, err := s.store.ListChecks(r.Context(), m.ID, checksPerPage, 0)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "monitor_detail", page{Title: m.Name, Nav: "monitors", Data: monitorDetail{
		Monitor: m,
		Uptime:  sum,
		Checks:  checks,
	}})
}
