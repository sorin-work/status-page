package web

import (
	"context"
	"net/http"

	"github.com/sorin-work/status-page/internal/component"
	"github.com/sorin-work/status-page/internal/incident"
	"github.com/sorin-work/status-page/internal/maintenance"
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
	Active     []incident.Incident
	Past       []incident.Incident
	// Maintenance holds windows that are running or coming up.
	Maintenance []maintenance.Window
}

// pastIncidentLimit is how many resolved incidents the status page lists.
const pastIncidentLimit = 10

func (s *Server) handleStatusPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.buildStatus(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "status", page{Data: data})
}

// buildStatus gathers everything shown on the public status page.
func (s *Server) buildStatus(ctx context.Context) (statusPage, error) {
	components, err := s.store.ListComponents(ctx)
	if err != nil {
		return statusPage{}, err
	}
	active, err := s.store.ListOpenIncidents(ctx)
	if err != nil {
		return statusPage{}, err
	}
	windows, err := s.store.ListCurrentMaintenance(ctx, s.now())
	if err != nil {
		return statusPage{}, err
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
			return statusPage{}, err
		}
		stats, err := s.store.ComponentDailyStats(ctx, c.ID, from, today)
		if err != nil {
			return statusPage{}, err
		}
		pc := publicComponent{Component: c, Level: component.LevelFor(monitors)}
		// Planned work explains failing checks, so it wins over them.
		for _, win := range windows {
			if win.Active(s.now()) && win.Covers(c.ID) {
				pc.Level = component.Maintenance
			}
		}
		for _, inc := range active {
			if l := component.ForIncident(inc.Impact); inc.ComponentID == c.ID && component.Worse(l, pc.Level) {
				pc.Level = l
			}
		}
		pc.Bars = uptime.Bars(stats, today, uptime.HistoryDays)
		for _, b := range pc.Bars {
			pc.Uptime = pc.Uptime.Add(b.Ratio)
		}
		levels = append(levels, pc.Level)
		data.Components = append(data.Components, pc)
	}
	// Incidents that are not tied to a component affect the whole page.
	for _, inc := range active {
		if inc.ComponentID == 0 {
			levels = append(levels, component.ForIncident(inc.Impact))
		}
	}
	past, err := s.store.ListRecentResolved(ctx, pastIncidentLimit)
	if err != nil {
		return statusPage{}, err
	}
	data.Maintenance = windows
	data.Active = active
	data.Past = past
	data.Overall = component.Overall(levels)
	data.Headline = component.Headline(data.Overall)
	return data, nil
}

func (s *Server) handlePublicIncident(w http.ResponseWriter, r *http.Request) {
	inc, ok := s.loadIncident(w, r)
	if !ok {
		return
	}
	var compName string
	if inc.ComponentID != 0 {
		if c, err := s.store.GetComponent(r.Context(), inc.ComponentID); err == nil {
			compName = c.Name
		}
	}
	s.render(w, r, http.StatusOK, "public_incident", page{Title: inc.Title, Data: struct {
		Incident  incident.Incident
		Component string
	}{inc, compName}})
}
