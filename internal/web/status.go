package web

import (
	"net/http"

	"github.com/sorin-work/status-page/internal/component"
	"github.com/sorin-work/status-page/internal/store"
	"github.com/sorin-work/status-page/internal/uptime"
)

type publicComponent struct {
	Component store.Component
	Level     component.Level
	Bars      []uptime.Bar
	Uptime    uptime.Ratio
}

type statusPage struct {
	Overall    component.Level
	Headline   string
	Components []publicComponent
}

func (s *Server) handleStatusPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	components, err := s.store.ListComponents(ctx)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	var (
		data   statusPage
		levels []component.Level
		today  = s.now().UTC()
		from   = today.AddDate(0, 0, -(uptime.HistoryDays - 1))
	)
	for _, c := range components {
		monitors, err := s.store.ListComponentMonitors(ctx, c.ID)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		stats, err := s.store.ComponentDailyStats(ctx, c.ID, from, today)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		pc := publicComponent{Component: c, Level: component.LevelFor(monitors)}
		pc.Bars = uptime.Bars(stats, today, uptime.HistoryDays)
		for _, b := range pc.Bars {
			pc.Uptime = pc.Uptime.Add(b.Ratio)
		}
		levels = append(levels, pc.Level)
		data.Components = append(data.Components, pc)
	}
	data.Overall = component.Overall(levels)
	data.Headline = component.Headline(data.Overall)
	s.render(w, r, http.StatusOK, "status", page{Data: data})
}
