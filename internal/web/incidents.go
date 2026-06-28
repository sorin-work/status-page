package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sorin-work/status-page/internal/incident"
	"github.com/sorin-work/status-page/internal/notify"
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
	Title       string
	Status      string
	Impact      string
	Message     string
	Errors      map[string]string
	Statuses    []incident.Status
	Impacts     []incident.Impact
	ComponentID int64
	Components  []store.Component
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

func (s *Server) renderIncidentForm(w http.ResponseWriter, r *http.Request, status int, f incidentForm) {
	components, err := s.store.ListComponents(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	f.Components = components
	s.render(w, r, status, "incident_form", page{Title: "New incident", Nav: "incidents", Data: f})
}

func (s *Server) handleNewIncident(w http.ResponseWriter, r *http.Request) {
	s.renderIncidentForm(w, r, http.StatusOK, newIncidentForm())
}

func (s *Server) handleCreateIncident(w http.ResponseWriter, r *http.Request) {
	f := newIncidentForm()
	f.Title = strings.TrimSpace(r.PostFormValue("title"))
	f.Status = r.PostFormValue("status")
	f.Impact = r.PostFormValue("impact")
	f.Message = r.PostFormValue("message")
	f.ComponentID, _ = strconv.ParseInt(r.PostFormValue("component_id"), 10, 64)

	status, impact := incident.Status(f.Status), incident.Impact(f.Impact)
	var verr incident.ValidationError
	if err := incident.ValidateNew(f.Title, status, impact, f.Message); errors.As(err, &verr) {
		f.Errors = verr
		s.renderIncidentForm(w, r, http.StatusUnprocessableEntity, f)
		return
	}
	if f.ComponentID != 0 {
		if _, err := s.store.GetComponent(r.Context(), f.ComponentID); err != nil {
			f.ComponentID = 0
		}
	}
	inc, err := s.store.CreateIncident(r.Context(), incident.Incident{
		Title:       f.Title,
		Status:      status,
		Impact:      impact,
		ComponentID: f.ComponentID,
	}, f.Message)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.publishIncident(r.Context(), notify.EventIncidentCreated, inc, inc.Status, f.Message)
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

type updateForm struct {
	Status  string
	Message string
	Errors  map[string]string
}

type incidentDetail struct {
	Incident incident.Incident
	Form     updateForm
	Statuses []incident.Status
}

func (s *Server) handleIncidentDetail(w http.ResponseWriter, r *http.Request) {
	inc, ok := s.loadIncident(w, r)
	if !ok {
		return
	}
	s.render(w, r, http.StatusOK, "incident_detail", page{Title: inc.Title, Nav: "incidents", Data: incidentDetail{
		Incident: inc, Form: updateForm{Status: string(inc.Status)}, Statuses: incident.Statuses,
	}})
}

func (s *Server) handleAddIncidentUpdate(w http.ResponseWriter, r *http.Request) {
	inc, ok := s.loadIncident(w, r)
	if !ok {
		return
	}
	status := incident.Status(r.PostFormValue("status"))
	message := r.PostFormValue("message")
	var verr incident.ValidationError
	if err := incident.ValidateUpdate(status, message); errors.As(err, &verr) {
		s.render(w, r, http.StatusUnprocessableEntity, "incident_detail", page{Title: inc.Title, Nav: "incidents", Data: incidentDetail{
			Incident: inc, Form: updateForm{Status: string(status), Message: message, Errors: verr}, Statuses: incident.Statuses,
		}})
		return
	}
	if _, err := s.store.AddIncidentUpdate(r.Context(), inc.ID, status, message, time.Time{}); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.publishIncident(r.Context(), notify.EventIncidentUpdated, inc, status, message)
	http.Redirect(w, r, "/admin/incidents/"+strconv.FormatInt(inc.ID, 10), http.StatusSeeOther)
}
