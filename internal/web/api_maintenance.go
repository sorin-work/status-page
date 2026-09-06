package web

import (
	"errors"
	"net/http"
	"time"

	"github.com/sorin-work/status-page/internal/maintenance"
	"github.com/sorin-work/status-page/internal/store"
)

type apiComponent struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Position    int    `json:"position"`
}

func (s *Server) apiListComponents(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListComponents(r.Context())
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	out := make([]apiComponent, 0, len(list))
	for _, c := range list {
		out = append(out, apiComponent{ID: c.ID, Name: c.Name, Description: c.Description, Position: c.Position})
	}
	writeJSON(w, http.StatusOK, map[string]any{"components": out})
}

func (s *Server) toAPIMaintenance(m maintenance.Window) apiMaintenance {
	return apiMaintenance{
		ID: m.ID, Title: m.Title, Description: m.Description, ComponentID: optionalID(m.ComponentID),
		StartsAt: m.StartsAt, EndsAt: m.EndsAt, Active: m.Active(s.now()),
	}
}

func (s *Server) apiListMaintenance(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListCurrentMaintenance(r.Context(), s.now())
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	out := make([]apiMaintenance, 0, len(list))
	for _, m := range list {
		out = append(out, s.toAPIMaintenance(m))
	}
	writeJSON(w, http.StatusOK, map[string]any{"maintenance": out})
}

type apiMaintenanceInput struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ComponentID *int64    `json:"component_id"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
}

func (s *Server) apiCreateMaintenance(w http.ResponseWriter, r *http.Request) {
	var in apiMaintenanceInput
	if !decodeJSON(w, r, &in) {
		return
	}
	win := maintenance.Window{Title: in.Title, Description: in.Description, StartsAt: in.StartsAt, EndsAt: in.EndsAt}
	fields := map[string]string{}
	var verr maintenance.ValidationError
	if err := win.Validate(s.now()); errors.As(err, &verr) {
		for k, v := range verr {
			fields[k] = v
		}
	}
	if in.ComponentID != nil && *in.ComponentID != 0 {
		if _, err := s.store.GetComponent(r.Context(), *in.ComponentID); err != nil {
			fields["component_id"] = "does not exist"
		}
		win.ComponentID = *in.ComponentID
	}
	if len(fields) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: "validation failed", Fields: fields})
		return
	}
	created, err := s.store.CreateMaintenance(r.Context(), win)
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, s.toAPIMaintenance(created))
}

func (s *Server) apiDeleteMaintenance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "maintenance window not found")
		return
	}
	err := s.store.DeleteMaintenance(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "maintenance window not found")
		return
	}
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
