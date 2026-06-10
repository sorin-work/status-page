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
