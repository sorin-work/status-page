package web

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
)

func TestMonitorDetail(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	m := addMonitor(t, st, "api")
	ctx := context.Background()
	if err := st.RecordCheck(ctx, monitor.Result{MonitorID: m.ID, CheckedAt: time.Now(), Up: false, Error: "connection refused"}); err != nil {
		t.Fatal(err)
	}
	c, _ := login(t, srv)
	rec := getWith(t, srv, "/admin/monitors/"+itoa(m.ID), c)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail: %d", rec.Code)
	}
	for _, want := range []string{"api", "connection refused", "0.00%"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("detail page is missing %q", want)
		}
	}
}

func TestMonitorDetailChartRange(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	m := addMonitor(t, st, "api")
	c, _ := login(t, srv)
	for q, active := range map[string]string{"": "24 hours", "?range=7d": "7 days", "?range=1y": "24 hours"} {
		body := getWith(t, srv, "/admin/monitors/"+itoa(m.ID)+q, c).Body.String()
		if !strings.Contains(body, `class="active">`+active) {
			t.Errorf("%q: %s is not the active range", q, active)
		}
	}
}

func TestMonitorDetailWithoutChecks(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	m := addMonitor(t, st, "api")
	c, _ := login(t, srv)
	rec := getWith(t, srv, "/admin/monitors/"+itoa(m.ID), c)
	if !strings.Contains(rec.Body.String(), "No checks have run yet") {
		t.Error("missing empty state")
	}
}
