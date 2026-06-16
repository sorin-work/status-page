package web

import (
	"errors"
	"net/http"
	"time"

	"github.com/sorin-work/status-page/internal/incident"
	"github.com/sorin-work/status-page/internal/store"
)

type apiUpdate struct {
	ID        int64     `json:"id"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type apiIncident struct {
	ID          int64       `json:"id"`
	Title       string      `json:"title"`
	Status      string      `json:"status"`
	Impact      string      `json:"impact"`
	ComponentID *int64      `json:"component_id"`
	MonitorID   *int64      `json:"monitor_id"`
	Automatic   bool        `json:"automatic"`
	StartedAt   time.Time   `json:"started_at"`
	ResolvedAt  *time.Time  `json:"resolved_at"`
	Updates     []apiUpdate `json:"updates,omitempty"`
}

func optionalID(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

func toAPIIncident(inc incident.Incident) apiIncident {
	out := apiIncident{
		ID:          inc.ID,
		Title:       inc.Title,
		Status:      string(inc.Status),
		Impact:      string(inc.Impact),
		ComponentID: optionalID(inc.ComponentID),
		MonitorID:   optionalID(inc.MonitorID),
		Automatic:   inc.Automatic,
		StartedAt:   inc.StartedAt,
	}
	if !inc.ResolvedAt.IsZero() {
		t := inc.ResolvedAt
		out.ResolvedAt = &t
	}
	for _, u := range inc.Updates {
		out.Updates = append(out.Updates, apiUpdate{ID: u.ID, Status: string(u.Status), Message: u.Message, CreatedAt: u.CreatedAt})
	}
	return out
}

func (s *Server) apiListIncidents(w http.ResponseWriter, r *http.Request) {
	limit, ok := queryInt(r, "limit", 50, 200)
	if !ok || limit == 0 {
		writeError(w, http.StatusBadRequest, "limit must be between 1 and 200")
		return
	}
	offset, ok := queryInt(r, "offset", 0, 1<<30)
	if !ok {
		writeError(w, http.StatusBadRequest, "offset must be a non-negative number")
		return
	}
	list, err := s.store.ListIncidents(r.Context(), limit, offset)
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	out := make([]apiIncident, 0, len(list))
	for _, inc := range list {
		out = append(out, toAPIIncident(inc))
	}
	writeJSON(w, http.StatusOK, map[string]any{"incidents": out})
}

func (s *Server) apiLoadIncident(w http.ResponseWriter, r *http.Request) (incident.Incident, bool) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "incident not found")
		return incident.Incident{}, false
	}
	inc, err := s.store.GetIncident(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "incident not found")
		return incident.Incident{}, false
	}
	if err != nil {
		s.apiServerError(w, r, err)
		return incident.Incident{}, false
	}
	return inc, true
}

func (s *Server) apiGetIncident(w http.ResponseWriter, r *http.Request) {
	inc, ok := s.apiLoadIncident(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toAPIIncident(inc))
}

type apiIncidentInput struct {
	Title       string `json:"title"`
	Status      string `json:"status"`
	Impact      string `json:"impact"`
	Message     string `json:"message"`
	ComponentID *int64 `json:"component_id"`
}

func (s *Server) apiCreateIncident(w http.ResponseWriter, r *http.Request) {
	var in apiIncidentInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.Status == "" {
		in.Status = string(incident.StatusInvestigating)
	}
	if in.Impact == "" {
		in.Impact = string(incident.ImpactMinor)
	}
	inc := incident.Incident{Title: in.Title, Status: incident.Status(in.Status), Impact: incident.Impact(in.Impact)}
	fields := map[string]string{}
	var verr incident.ValidationError
	if err := incident.ValidateNew(inc.Title, inc.Status, inc.Impact, in.Message); errors.As(err, &verr) {
		for k, v := range verr {
			fields[k] = v
		}
	}
	if in.ComponentID != nil && *in.ComponentID != 0 {
		if _, err := s.store.GetComponent(r.Context(), *in.ComponentID); err != nil {
			fields["component_id"] = "does not exist"
		}
		inc.ComponentID = *in.ComponentID
	}
	if len(fields) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: "validation failed", Fields: fields})
		return
	}
	created, err := s.store.CreateIncident(r.Context(), inc, in.Message)
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/incidents/"+itoa64(created.ID))
	writeJSON(w, http.StatusCreated, toAPIIncident(created))
}

type apiUpdateInput struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (s *Server) apiAddIncidentUpdate(w http.ResponseWriter, r *http.Request) {
	inc, ok := s.apiLoadIncident(w, r)
	if !ok {
		return
	}
	var in apiUpdateInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.Status == "" {
		in.Status = string(inc.Status)
	}
	var verr incident.ValidationError
	if err := incident.ValidateUpdate(incident.Status(in.Status), in.Message); errors.As(err, &verr) {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: "validation failed", Fields: verr})
		return
	}
	u, err := s.store.AddIncidentUpdate(r.Context(), inc.ID, incident.Status(in.Status), in.Message, s.now())
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, apiUpdate{ID: u.ID, Status: string(u.Status), Message: u.Message, CreatedAt: u.CreatedAt})
}
