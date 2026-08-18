package web

import (
	"net/http"
	"strings"

	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/uptime"
)

type dashboardRow struct {
	Monitor monitor.Monitor
	Uptime  uptime.Summary
	Last    *monitor.Result
}

type dashboardData struct {
	Rows   []dashboardRow
	Up     int
	Down   int
	Total  int
	Query  string
	Filter string
}

// matches applies the search box and status filter of the dashboard.
func matches(m monitor.Monitor, query, filter string) bool {
	if query != "" {
		q := strings.ToLower(query)
		if !strings.Contains(strings.ToLower(m.Name), q) && !strings.Contains(strings.ToLower(m.Target), q) {
			return false
		}
	}
	switch filter {
	case "up":
		return !m.Paused && m.Status == monitor.StatusUp
	case "down":
		return !m.Paused && m.Status == monitor.StatusDown
	case "paused":
		return m.Paused
	}
	return true
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	monitors, err := s.store.ListMonitors(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	latest, err := s.store.LatestChecks(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	data := dashboardData{
		Total:  len(monitors),
		Query:  strings.TrimSpace(r.URL.Query().Get("q")),
		Filter: r.URL.Query().Get("status"),
	}
	now := s.now()
	for _, m := range monitors {
		switch {
		case m.Paused:
		case m.Status == monitor.StatusUp:
			data.Up++
		case m.Status == monitor.StatusDown:
			data.Down++
		}
		if !matches(m, data.Query, data.Filter) {
			continue
		}
		sum, err := uptime.Summarize(r.Context(), s.store, m.ID, now)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		row := dashboardRow{Monitor: m, Uptime: sum}
		if last, ok := latest[m.ID]; ok {
			row.Last = &last
		}
		data.Rows = append(data.Rows, row)
	}
	s.render(w, r, http.StatusOK, "dashboard", page{Title: "Monitors", Nav: "monitors", Data: data})
}
