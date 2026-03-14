package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/store"
)

// monitorForm keeps the raw form values so they can be shown again next
// to validation errors.
type monitorForm struct {
	ID             int64
	Name           string
	Type           string
	Target         string
	Interval       string
	Timeout        string
	ExpectedStatus string
	Keyword        string
	Retries        string
	Paused         bool
	Errors         map[string]string
}

func formFromMonitor(m monitor.Monitor) monitorForm {
	f := monitorForm{
		ID:       m.ID,
		Name:     m.Name,
		Type:     string(m.Type),
		Target:   m.Target,
		Interval: strconv.Itoa(int(m.Interval / time.Second)),
		Timeout:  strconv.Itoa(int(m.Timeout / time.Second)),
		Keyword:  m.Keyword,
		Retries:  strconv.Itoa(m.Retries),
		Paused:   m.Paused,
	}
	if m.ExpectedStatus != 0 {
		f.ExpectedStatus = strconv.Itoa(m.ExpectedStatus)
	}
	return f
}

func newMonitorForm() monitorForm {
	return formFromMonitor(monitor.Monitor{
		Type:     monitor.TypeHTTP,
		Interval: time.Minute,
		Timeout:  10 * time.Second,
		Retries:  1,
	})
}

// parseMonitorForm reads the form into a monitor and validates it.
func parseMonitorForm(r *http.Request) (monitor.Monitor, monitorForm, bool) {
	f := monitorForm{
		Name:           r.PostFormValue("name"),
		Type:           r.PostFormValue("type"),
		Target:         r.PostFormValue("target"),
		Interval:       strings.TrimSpace(r.PostFormValue("interval")),
		Timeout:        strings.TrimSpace(r.PostFormValue("timeout")),
		ExpectedStatus: strings.TrimSpace(r.PostFormValue("expected_status")),
		Keyword:        r.PostFormValue("keyword"),
		Retries:        strings.TrimSpace(r.PostFormValue("retries")),
		Paused:         r.PostFormValue("paused") == "on",
		Errors:         map[string]string{},
	}
	m := monitor.Monitor{
		Name:    f.Name,
		Type:    monitor.Type(f.Type),
		Target:  f.Target,
		Keyword: f.Keyword,
		Paused:  f.Paused,
	}
	number := func(field, value string, allowEmpty bool) int {
		if value == "" && allowEmpty {
			return 0
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			f.Errors[field] = "must be a whole number"
		}
		return n
	}
	m.Interval = time.Duration(number("interval", f.Interval, false)) * time.Second
	m.Timeout = time.Duration(number("timeout", f.Timeout, false)) * time.Second
	m.ExpectedStatus = number("expected_status", f.ExpectedStatus, true)
	m.Retries = number("retries", f.Retries, true)

	m.Normalize()
	var verr monitor.ValidationError
	if err := m.Validate(); errors.As(err, &verr) {
		for k, v := range verr {
			if _, ok := f.Errors[k]; !ok {
				f.Errors[k] = v
			}
		}
	}
	return m, f, len(f.Errors) == 0
}

func (s *Server) handleNewMonitor(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "monitor_form", page{Title: "New monitor", Nav: "monitors", Data: newMonitorForm()})
}

func (s *Server) handleCreateMonitor(w http.ResponseWriter, r *http.Request) {
	m, form, ok := parseMonitorForm(r)
	if !ok {
		s.render(w, r, http.StatusUnprocessableEntity, "monitor_form", page{Title: "New monitor", Nav: "monitors", Data: form})
		return
	}
	if _, err := s.store.CreateMonitor(r.Context(), m); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// pathID parses the {id} path value.
func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

// loadMonitor fetches the monitor named in the path or writes a 404.
func (s *Server) loadMonitor(w http.ResponseWriter, r *http.Request) (monitor.Monitor, bool) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return monitor.Monitor{}, false
	}
	m, err := s.store.GetMonitor(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return monitor.Monitor{}, false
	}
	if err != nil {
		s.serverError(w, r, err)
		return monitor.Monitor{}, false
	}
	return m, true
}

func (s *Server) handleEditMonitor(w http.ResponseWriter, r *http.Request) {
	m, ok := s.loadMonitor(w, r)
	if !ok {
		return
	}
	s.render(w, r, http.StatusOK, "monitor_form", page{Title: "Edit " + m.Name, Nav: "monitors", Data: formFromMonitor(m)})
}

func (s *Server) handleUpdateMonitor(w http.ResponseWriter, r *http.Request) {
	existing, ok := s.loadMonitor(w, r)
	if !ok {
		return
	}
	m, form, valid := parseMonitorForm(r)
	form.ID = existing.ID
	if !valid {
		s.render(w, r, http.StatusUnprocessableEntity, "monitor_form", page{Title: "Edit " + existing.Name, Nav: "monitors", Data: form})
		return
	}
	m.ID = existing.ID
	if _, err := s.store.UpdateMonitor(r.Context(), m); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}
