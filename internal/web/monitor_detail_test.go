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

func TestCheckHistoryLastPage(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	m := addMonitor(t, st, "api")
	start := time.Now().Add(-3 * time.Hour)
	var batch []monitor.Result
	for i := range 120 {
		batch = append(batch, monitor.Result{MonitorID: m.ID, CheckedAt: start.Add(time.Duration(i) * time.Minute), Up: true, Latency: time.Duration(i+1) * time.Millisecond})
	}
	if err := st.RecordChecks(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	c, _ := login(t, srv)
	body := getWith(t, srv, "/admin/monitors/"+itoa(m.ID)+"?page=3", c).Body.String()
	if !strings.Contains(body, "Page 3 of 3") {
		t.Fatal("the last, partial page is not reachable")
	}
	// The oldest check (1ms) is on the last page.
	if !strings.Contains(body, ">1ms<") {
		t.Error("oldest check missing from the last page")
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
