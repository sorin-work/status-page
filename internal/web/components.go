package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sorin-work/status-page/internal/store"
)

type componentsData struct {
	Components []store.Component
	Monitors   map[int64]int
	Error      string
	Name       string
	Desc       string
}

func (s *Server) renderComponents(w http.ResponseWriter, r *http.Request, status int, data componentsData) {
	list, err := s.store.ListComponents(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	data.Components = list
	if data.Monitors, err = s.store.CountComponentMonitors(r.Context()); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, status, "components", page{Title: "Components", Nav: "components", Data: data})
}

func (s *Server) handleComponents(w http.ResponseWriter, r *http.Request) {
	s.renderComponents(w, r, http.StatusOK, componentsData{})
}

func validateComponent(name, desc string) string {
	switch {
	case name == "":
		return "Name is required."
	case utf8.RuneCountInString(name) > 100:
		return "Name must be at most 100 characters."
	case utf8.RuneCountInString(desc) > 500:
		return "Description must be at most 500 characters."
	}
	return ""
}

func (s *Server) handleCreateComponent(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PostFormValue("name"))
	desc := strings.TrimSpace(r.PostFormValue("description"))
	if msg := validateComponent(name, desc); msg != "" {
		s.renderComponents(w, r, http.StatusUnprocessableEntity, componentsData{Error: msg, Name: name, Desc: desc})
		return
	}
	if _, err := s.store.CreateComponent(r.Context(), store.Component{Name: name, Description: desc}); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/components", http.StatusSeeOther)
}

func (s *Server) handleUpdateComponent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	c, err := s.store.GetComponent(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		s.notFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	c.Name = strings.TrimSpace(r.PostFormValue("name"))
	c.Description = strings.TrimSpace(r.PostFormValue("description"))
	if pos, err := strconv.Atoi(r.PostFormValue("position")); err == nil {
		c.Position = pos
	}
	if msg := validateComponent(c.Name, c.Description); msg != "" {
		s.renderComponents(w, r, http.StatusUnprocessableEntity, componentsData{Error: msg})
		return
	}
	if err := s.store.UpdateComponent(r.Context(), c); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/components", http.StatusSeeOther)
}

func (s *Server) handleDeleteComponent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return
	}
	err := s.store.DeleteComponent(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		s.notFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/components", http.StatusSeeOther)
}
