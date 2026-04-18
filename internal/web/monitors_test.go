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
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/admin/monitors/") {
		t.Fatalf("create: %d to %q", rec.Code, rec.Header().Get("Location"))
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

func TestCreateMonitorWithComponent(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	comp, err := st.CreateComponent(context.Background(), store.Component{Name: "API"})
	if err != nil {
		t.Fatal(err)
	}
	c, csrf := login(t, srv)
	if body := getWith(t, srv, "/admin/monitors/new", c).Body.String(); !strings.Contains(body, ">API</option>") {
		t.Error("component missing from the form")
	}
	form := monitorValues(csrf)
	form.Set("component_id", itoa(comp.ID))
	if rec := postForm(t, srv, "/admin/monitors", form, c); rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d", rec.Code)
	}
	list, _ := st.ListMonitors(context.Background())
	if len(list) != 1 || list[0].ComponentID != comp.ID {
		t.Errorf("monitor = %+v", list)
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

func TestPauseAndResumeMonitor(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	m := addMonitor(t, st, "api")
	c, csrf := login(t, srv)
	ctx := context.Background()

	rec := postForm(t, srv, "/admin/monitors/"+itoa(m.ID)+"/pause", url.Values{"csrf": {csrf}}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("pause: %d", rec.Code)
	}
	if got, _ := st.GetMonitor(ctx, m.ID); !got.Paused {
		t.Fatal("monitor not paused")
	}
	if body := getWith(t, srv, "/admin/monitors/"+itoa(m.ID), c).Body.String(); !strings.Contains(body, "Resume") {
		t.Error("paused monitor page has no resume button")
	}
	postForm(t, srv, "/admin/monitors/"+itoa(m.ID)+"/resume", url.Values{"csrf": {csrf}}, c)
	if got, _ := st.GetMonitor(ctx, m.ID); got.Paused {
		t.Fatal("monitor still paused")
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
