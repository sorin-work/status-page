package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestMaintenancePage(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, _ := login(t, srv)
	rec := getWith(t, srv, "/admin/maintenance", c)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "No maintenance planned") {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestScheduleMaintenance(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, csrf := login(t, srv)
	start := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Minute)

	rec := postForm(t, srv, "/admin/maintenance", url.Values{
		"csrf": {csrf}, "title": {"Database upgrade"}, "description": {"Read-only for 30 minutes."},
		"starts_at": {start.Format(datetimeLocal)}, "ends_at": {start.Add(30 * time.Minute).Format(datetimeLocal)},
	}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	list, err := st.ListCurrentMaintenance(context.Background(), time.Now())
	if err != nil || len(list) != 1 {
		t.Fatalf("windows = %+v %v", list, err)
	}
	if !list[0].StartsAt.Equal(start) || list[0].EndsAt.Sub(list[0].StartsAt) != 30*time.Minute {
		t.Errorf("window = %+v", list[0])
	}
	if body := getWith(t, srv, "/admin/maintenance", c).Body.String(); !strings.Contains(body, "Database upgrade") {
		t.Error("window not listed")
	}
}

func TestScheduleMaintenanceValidation(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, csrf := login(t, srv)
	rec := postForm(t, srv, "/admin/maintenance", url.Values{
		"csrf": {csrf}, "title": {""}, "starts_at": {"tomorrow"}, "ends_at": {"2026-01-01T10:00"},
	}, c)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Title is required", "Start is not a valid date and time"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
}
