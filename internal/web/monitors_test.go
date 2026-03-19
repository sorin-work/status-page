package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/store"
)

func monitorValues(csrf string) url.Values {
	return url.Values{
		"csrf":     {csrf},
		"name":     {"Public API"},
		"type":     {"http"},
		"target":   {"https://api.example.com/health"},
		"interval": {"30"},
		"timeout":  {"5"},
		"retries":  {"2"},
		"keyword":  {"ok"},
	}
}

func TestNewMonitorForm(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, _ := login(t, srv)
	rec := getWith(t, srv, "/admin/monitors/new", c)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Create monitor") {
		t.Fatalf("new form: %d", rec.Code)
	}
}

func TestCreateMonitor(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, csrf := login(t, srv)

	rec := postForm(t, srv, "/admin/monitors", monitorValues(csrf), c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	list, err := st.ListMonitors(context.Background())
	if err != nil || len(list) != 1 {
		t.Fatalf("monitors: %v %v", list, err)
	}
	m := list[0]
	if m.Name != "Public API" || m.Interval != 30*time.Second || m.Timeout != 5*time.Second || m.Retries != 2 || m.Keyword != "ok" {
		t.Errorf("saved monitor: %+v", m)
	}
}

func TestCreateMonitorValidation(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, csrf := login(t, srv)

	form := monitorValues(csrf)
	form.Set("target", "not a url")
	form.Set("interval", "soon")
	rec := postForm(t, srv, "/admin/monitors", form, c)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Target must start with http", "Interval must be a whole number", `value="Public API"`} {
		if !strings.Contains(body, want) {
			t.Errorf("response is missing %q", want)
		}
	}
	if list, _ := st.ListMonitors(context.Background()); len(list) != 0 {
		t.Errorf("invalid monitor was saved")
	}
}

func TestCreateMonitorRequiresCSRF(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, _ := login(t, srv)
	if rec := postForm(t, srv, "/admin/monitors", monitorValues("wrong"), c); rec.Code != http.StatusForbidden {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestEditMonitor(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	m := addMonitor(t, st, "website")
	c, csrf := login(t, srv)

	rec := getWith(t, srv, "/admin/monitors/"+itoa(m.ID)+"/edit", c)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `value="website"`) {
		t.Fatalf("edit form: %d", rec.Code)
	}

	form := monitorValues(csrf)
	form.Set("name", "Website")
	rec = postForm(t, srv, "/admin/monitors/"+itoa(m.ID), form, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update: %d", rec.Code)
	}
	got, _ := st.GetMonitor(context.Background(), m.ID)
	if got.Name != "Website" || got.Interval != 30*time.Second {
		t.Errorf("not updated: %+v", got)
	}
}

func TestEditMissingMonitor(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, _ := login(t, srv)
	for _, p := range []string{"/admin/monitors/999/edit", "/admin/monitors/abc/edit"} {
		if rec := getWith(t, srv, p, c); rec.Code != http.StatusNotFound {
			t.Errorf("%s: got %d", p, rec.Code)
		}
	}
}

func TestDeleteMonitor(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	m := addMonitor(t, st, "old")
	c, csrf := login(t, srv)

	rec := postForm(t, srv, "/admin/monitors/"+itoa(m.ID)+"/delete", url.Values{"csrf": {csrf}}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d", rec.Code)
	}
	if _, err := st.GetMonitor(context.Background(), m.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("monitor still exists")
	}
}
