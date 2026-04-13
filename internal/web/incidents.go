package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/sorin-work/status-page/internal/incident"
	"github.com/sorin-work/status-page/internal/store"
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

type incidentForm struct {
	Title    string
	Status   string
	Impact   string
	Message  string
	Errors   map[string]string
	Statuses []incident.Status
	Impacts  []incident.Impact
}

func newIncidentForm() incidentForm {
	return incidentForm{
		Status:   string(incident.StatusInvestigating),
		Impact:   string(incident.ImpactMinor),
		Errors:   map[string]string{},
		Statuses: incident.Statuses,
		Impacts:  incident.Impacts,
	}
}

func (s *Server) handleNewIncident(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "incident_form", page{Title: "New incident", Nav: "incidents", Data: newIncidentForm()})
}

func (s *Server) handleCreateIncident(w http.ResponseWriter, r *http.Request) {
	f := newIncidentForm()
	f.Title = strings.TrimSpace(r.PostFormValue("title"))
	f.Status = r.PostFormValue("status")
	f.Impact = r.PostFormValue("impact")
	f.Message = r.PostFormValue("message")

	status, impact := incident.Status(f.Status), incident.Impact(f.Impact)
	var verr incident.ValidationError
	if err := incident.ValidateNew(f.Title, status, impact, f.Message); errors.As(err, &verr) {
		f.Errors = verr
		s.render(w, r, http.StatusUnprocessableEntity, "incident_form", page{Title: "New incident", Nav: "incidents", Data: f})
		return
	}
	inc, err := s.store.CreateIncident(r.Context(), incident.Incident{
		Title:  f.Title,
		Status: status,
		Impact: impact,
	}, f.Message)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/incidents/"+strconv.FormatInt(inc.ID, 10), http.StatusSeeOther)
}

func (s *Server) loadIncident(w http.ResponseWriter, r *http.Request) (incident.Incident, bool) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return incident.Incident{}, false
	}
	inc, err := s.store.GetIncident(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return incident.Incident{}, false
	}
	if err != nil {
		s.serverError(w, r, err)
		return incident.Incident{}, false
	}
	return inc, true
}

type incidentDetail struct {
	Incident incident.Incident
}

func (s *Server) handleIncidentDetail(w http.ResponseWriter, r *http.Request) {
	inc, ok := s.loadIncident(w, r)
	if !ok {
		return
	}
	s.render(w, r, http.StatusOK, "incident_detail", page{Title: inc.Title, Nav: "incidents", Data: incidentDetail{Incident: inc}})
}
