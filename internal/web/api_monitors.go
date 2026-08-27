package web

import (
	"errors"
	"net/http"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/store"
)

type apiMonitor struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Type            string    `json:"type"`
	Target          string    `json:"target"`
	IntervalSeconds int       `json:"interval_seconds"`
	TimeoutSeconds  int       `json:"timeout_seconds"`
	ExpectedStatus  int       `json:"expected_status"`
	Keyword         string    `json:"keyword"`
	Retries         int       `json:"retries"`
	Paused          bool      `json:"paused"`
	Status          string    `json:"status"`
	ComponentID     *int64    `json:"component_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func toAPIMonitor(m monitor.Monitor) apiMonitor {
	out := apiMonitor{
		ID:              m.ID,
		Name:            m.Name,
		Type:            string(m.Type),
		Target:          m.Target,
		IntervalSeconds: int(m.Interval / time.Second),
		TimeoutSeconds:  int(m.Timeout / time.Second),
		ExpectedStatus:  m.ExpectedStatus,
		Keyword:         m.Keyword,
		Retries:         m.Retries,
		Paused:          m.Paused,
		Status:          string(m.Status),
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
	if m.ComponentID != 0 {
		id := m.ComponentID
		out.ComponentID = &id
	}
	return out
}

func (s *Server) apiListMonitors(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListMonitors(r.Context())
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	out := make([]apiMonitor, 0, len(list))
	for _, m := range list {
		out = append(out, toAPIMonitor(m))
	}
	writeJSON(w, http.StatusOK, map[string]any{"monitors": out})
}

// apiLoadMonitor fetches the monitor in the path or writes a 404.
func (s *Server) apiLoadMonitor(w http.ResponseWriter, r *http.Request) (monitor.Monitor, bool) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "monitor not found")
		return monitor.Monitor{}, false
	}
	m, err := s.store.GetMonitor(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return monitor.Monitor{}, false
	}
	if err != nil {
		s.apiServerError(w, r, err)
		return monitor.Monitor{}, false
	}
	return m, true
}

func (s *Server) apiGetMonitor(w http.ResponseWriter, r *http.Request) {
	m, ok := s.apiLoadMonitor(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toAPIMonitor(m))
}

type apiMonitorInput struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	Target          string `json:"target"`
	IntervalSeconds int    `json:"interval_seconds"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
	ExpectedStatus  int    `json:"expected_status"`
	Keyword         string `json:"keyword"`
	Retries         int    `json:"retries"`
	Paused          bool   `json:"paused"`
	ComponentID     *int64 `json:"component_id"`
}

// toMonitor applies defaults and validates the input. It writes the error
// response itself and returns false when the input is rejected.
func (s *Server) toMonitor(w http.ResponseWriter, r *http.Request, in apiMonitorInput) (monitor.Monitor, bool) {
	if in.Type == "" {
		in.Type = string(monitor.TypeHTTP)
	}
	if in.IntervalSeconds == 0 {
		in.IntervalSeconds = 60
	}
	if in.TimeoutSeconds == 0 {
		in.TimeoutSeconds = 10
	}
	m := monitor.Monitor{
		Name:           in.Name,
		Type:           monitor.Type(in.Type),
		Target:         in.Target,
		Interval:       time.Duration(in.IntervalSeconds) * time.Second,
		Timeout:        time.Duration(in.TimeoutSeconds) * time.Second,
		ExpectedStatus: in.ExpectedStatus,
		Keyword:        in.Keyword,
		Retries:        in.Retries,
		Paused:         in.Paused,
	}
	m.Normalize()
	fields := map[string]string{}
	var verr monitor.ValidationError
	if err := m.Validate(); errors.As(err, &verr) {
		for k, v := range verr {
			fields[k] = v
		}
	}
	if in.ComponentID != nil && *in.ComponentID != 0 {
		if _, err := s.store.GetComponent(r.Context(), *in.ComponentID); errors.Is(err, store.ErrNotFound) {
			fields["component_id"] = "does not exist"
		} else if err != nil {
			s.apiServerError(w, r, err)
			return monitor.Monitor{}, false
		}
		m.ComponentID = *in.ComponentID
	}
	if len(fields) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: "validation failed", Fields: fields})
		return monitor.Monitor{}, false
	}
	return m, true
}

func (s *Server) apiCreateMonitor(w http.ResponseWriter, r *http.Request) {
	var in apiMonitorInput
	if !decodeJSON(w, r, &in) {
		return
	}
	m, ok := s.toMonitor(w, r, in)
	if !ok {
		return
	}
	created, err := s.store.CreateMonitor(r.Context(), m)
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/monitors/"+itoa64(created.ID))
	writeJSON(w, http.StatusCreated, toAPIMonitor(created))
}

func (s *Server) apiUpdateMonitor(w http.ResponseWriter, r *http.Request) {
	existing, ok := s.apiLoadMonitor(w, r)
	if !ok {
		return
	}
	var in apiMonitorInput
	if !decodeJSON(w, r, &in) {
		return
	}
	m, ok := s.toMonitor(w, r, in)
	if !ok {
		return
	}
	m.ID = existing.ID
	updated, err := s.store.UpdateMonitor(r.Context(), m)
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAPIMonitor(updated))
}

func (s *Server) apiDeleteMonitor(w http.ResponseWriter, r *http.Request) {
	m, ok := s.apiLoadMonitor(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteMonitor(r.Context(), m.ID); err != nil {
		s.apiServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiPauseMonitor(w http.ResponseWriter, r *http.Request) {
	s.apiSetPaused(w, r, true)
}

func (s *Server) apiResumeMonitor(w http.ResponseWriter, r *http.Request) {
	s.apiSetPaused(w, r, false)
}

func (s *Server) apiSetPaused(w http.ResponseWriter, r *http.Request, paused bool) {
	m, ok := s.apiLoadMonitor(w, r)
	if !ok {
		return
	}
	if err := s.store.SetMonitorPaused(r.Context(), m.ID, paused); err != nil {
		s.apiServerError(w, r, err)
		return
	}
	m, err := s.store.GetMonitor(r.Context(), m.ID)
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAPIMonitor(m))
}
