package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/maintenance"
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

func TestScheduleMaintenanceInDisplayTimeZone(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		t.Skip("no tz database")
	}
	srv, st := newTestServer(t)
	srv.loc = loc
	createAdmin(t, st)
	c, csrf := login(t, srv)

	day := time.Now().In(loc).AddDate(0, 0, 2)
	start := time.Date(day.Year(), day.Month(), day.Day(), 2, 0, 0, 0, loc)
	rec := postForm(t, srv, "/admin/maintenance", url.Values{
		"csrf": {csrf}, "title": {"Night upgrade"},
		"starts_at": {start.Format(datetimeLocal)}, "ends_at": {start.Add(time.Hour).Format(datetimeLocal)},
	}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d", rec.Code)
	}
	list, _ := st.ListCurrentMaintenance(context.Background(), time.Now())
	if len(list) != 1 || !list[0].StartsAt.Equal(start) {
		t.Fatalf("02:00 Bucharest stored as %v, want %v", list[0].StartsAt, start.UTC())
	}
	// And it is shown back as 02:00 local time.
	if body := getWith(t, srv, "/admin/maintenance", c).Body.String(); !strings.Contains(body, start.Format("2006-01-02")+" 02:00") {
		t.Error("window not shown at the time it was entered")
	}
}

func TestEndAndDeleteMaintenance(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, csrf := login(t, srv)
	ctx := context.Background()
	now := time.Now()
	running, _ := st.CreateMaintenance(ctx, maintenance.Window{Title: "running", StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour)})
	later, _ := st.CreateMaintenance(ctx, maintenance.Window{Title: "later", StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour)})

	if rec := postForm(t, srv, "/admin/maintenance/"+itoa(running.ID)+"/end", url.Values{"csrf": {csrf}}, c); rec.Code != http.StatusSeeOther {
		t.Fatalf("end: %d", rec.Code)
	}
	if rec := postForm(t, srv, "/admin/maintenance/"+itoa(later.ID)+"/delete", url.Values{"csrf": {csrf}}, c); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d", rec.Code)
	}
	if list, _ := st.ListCurrentMaintenance(ctx, time.Now().Add(time.Second)); len(list) != 0 {
		t.Errorf("windows left: %+v", list)
	}
	if rec := postForm(t, srv, "/admin/maintenance/"+itoa(later.ID)+"/delete", url.Values{"csrf": {csrf}}, c); rec.Code != http.StatusNotFound {
		t.Errorf("deleting twice: %d", rec.Code)
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
