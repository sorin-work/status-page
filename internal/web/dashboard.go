package web

import (
	"net/http"

	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/uptime"
)

type dashboardRow struct {
	Monitor monitor.Monitor
	Uptime  uptime.Summary
}

type dashboardData struct {
	Rows []dashboardRow
	Up   int
	Down int
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	monitors, err := s.store.ListMonitors(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	var data dashboardData
	now := s.now()
	for _, m := range monitors {
		sum, err := uptime.Summarize(r.Context(), s.store, m.ID, now)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		data.Rows = append(data.Rows, dashboardRow{Monitor: m, Uptime: sum})
		switch {
		case m.Paused:
		case m.Status == monitor.StatusUp:
			data.Up++
		case m.Status == monitor.StatusDown:
			data.Down++
		}
	}
	s.render(w, r, http.StatusOK, "dashboard", page{Title: "Monitors", Nav: "monitors", Data: data})
}
