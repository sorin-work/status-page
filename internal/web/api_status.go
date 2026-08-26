package web

import (
	"net/http"
	"time"
)

type apiComponentStatus struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Status      string   `json:"status"`
	Uptime90d   apiRatio `json:"uptime_90d"`
}

type apiMaintenance struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	ComponentID *int64    `json:"component_id"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	Active      bool      `json:"active"`
}

// apiStatus is the public summary of the status page. It needs no token.
func (s *Server) apiStatus(w http.ResponseWriter, r *http.Request) {
	data, err := s.statusCache.get(r.Context(), s.buildStatus)
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	components := make([]apiComponentStatus, 0, len(data.Components))
	for _, c := range data.Components {
		components = append(components, apiComponentStatus{
			ID:          c.Component.ID,
			Name:        c.Component.Name,
			Description: c.Component.Description,
			Status:      string(c.Level),
			Uptime90d:   toAPIRatio(c.Uptime),
		})
	}
	incidents := make([]apiIncident, 0, len(data.Active))
	for _, inc := range data.Active {
		incidents = append(incidents, toAPIIncident(inc))
	}
	windows := make([]apiMaintenance, 0, len(data.Maintenance))
	now := s.now()
	for _, m := range data.Maintenance {
		windows = append(windows, apiMaintenance{
			ID: m.ID, Title: m.Title, Description: m.Description, ComponentID: optionalID(m.ComponentID),
			StartsAt: m.StartsAt, EndsAt: m.EndsAt, Active: m.Active(now),
		})
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      string(data.Overall),
		"headline":    data.Headline,
		"components":  components,
		"incidents":   incidents,
		"maintenance": windows,
		"updated_at":  now.UTC().Truncate(time.Second),
	})
}
