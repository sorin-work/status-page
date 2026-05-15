package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sorin-work/status-page/internal/maintenance"
	"github.com/sorin-work/status-page/internal/store"
)

// datetimeLocal is the value format of <input type="datetime-local">.
const datetimeLocal = "2006-01-02T15:04"

type maintenanceForm struct {
	Title       string
	Description string
	ComponentID int64
	StartsAt    string
	EndsAt      string
	Errors      map[string]string
}

type maintenanceData struct {
	Current    []maintenance.Window
	Past       []maintenance.Window
	Components map[int64]string
	Choices    []store.Component
	Form       maintenanceForm
	Zone       string
}

func (s *Server) renderMaintenance(w http.ResponseWriter, r *http.Request, status int, f maintenanceForm) {
	ctx := r.Context()
	now := s.now()
	current, err := s.store.ListCurrentMaintenance(ctx, now)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	past, err := s.store.ListPastMaintenance(ctx, now, 20)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	components, err := s.store.ListComponents(ctx)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	names := map[int64]string{}
	for _, c := range components {
		names[c.ID] = c.Name
	}
	if f.Errors == nil {
		f.Errors = map[string]string{}
	}
	s.render(w, r, status, "maintenance", page{Title: "Maintenance", Nav: "maintenance", Data: maintenanceData{
		Current: current, Past: past, Components: names, Choices: components, Form: f, Zone: s.loc.String(),
	}})
}

func (s *Server) handleMaintenance(w http.ResponseWriter, r *http.Request) {
	start := s.now().In(s.loc).Add(time.Hour).Truncate(time.Hour)
	s.renderMaintenance(w, r, http.StatusOK, maintenanceForm{
		StartsAt: start.Format(datetimeLocal),
		EndsAt:   start.Add(time.Hour).Format(datetimeLocal),
	})
}

func (s *Server) handleCreateMaintenance(w http.ResponseWriter, r *http.Request) {
	f := maintenanceForm{
		Title:       strings.TrimSpace(r.PostFormValue("title")),
		Description: strings.TrimSpace(r.PostFormValue("description")),
		StartsAt:    r.PostFormValue("starts_at"),
		EndsAt:      r.PostFormValue("ends_at"),
		Errors:      map[string]string{},
	}
	f.ComponentID, _ = strconv.ParseInt(r.PostFormValue("component_id"), 10, 64)

	win := maintenance.Window{Title: f.Title, Description: f.Description, ComponentID: f.ComponentID}
	var err error
	if f.StartsAt != "" {
		if win.StartsAt, err = time.Parse(datetimeLocal, f.StartsAt); err != nil {
			f.Errors["starts_at"] = "is not a valid date and time"
		}
	}
	if f.EndsAt != "" {
		if win.EndsAt, err = time.Parse(datetimeLocal, f.EndsAt); err != nil {
			f.Errors["ends_at"] = "is not a valid date and time"
		}
	}
	var verr maintenance.ValidationError
	if err := win.Validate(s.now()); errors.As(err, &verr) {
		for k, v := range verr {
			if _, ok := f.Errors[k]; !ok {
				f.Errors[k] = v
			}
		}
	}
	if win.ComponentID != 0 {
		if _, err := s.store.GetComponent(r.Context(), win.ComponentID); err != nil {
			f.Errors["component_id"] = "does not exist"
		}
	}
	if len(f.Errors) > 0 {
		s.renderMaintenance(w, r, http.StatusUnprocessableEntity, f)
		return
	}
	if _, err := s.store.CreateMaintenance(r.Context(), win); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/maintenance", http.StatusSeeOther)
}
