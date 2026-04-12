package web

import (
	"net/http"

	"github.com/sorin-work/status-page/internal/incident"
)

const incidentsPerPage = 25

type incidentList struct {
	Incidents []incident.Incident
	Pager     pager
}

func (s *Server) handleIncidents(w http.ResponseWriter, r *http.Request) {
	total, err := s.store.CountIncidents(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	pg := newPager(r, total, incidentsPerPage)
	list, err := s.store.ListIncidents(r.Context(), pg.PerPage, pg.Offset())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "incidents", page{Title: "Incidents", Nav: "incidents", Data: incidentList{Incidents: list, Pager: pg}})
}
