package web

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/store"
)

func addMonitor(t *testing.T, st *store.Store, name string) monitor.Monitor {
	t.Helper()
	m, err := st.CreateMonitor(context.Background(), monitor.Monitor{
		Name: name, Type: monitor.TypeHTTP, Target: "https://example.com/" + name,
		Interval: time.Minute, Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestDashboardEmpty(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, _ := login(t, srv)
	body := getWith(t, srv, "/admin", c).Body.String()
	if !strings.Contains(body, "No monitors yet") {
		t.Error("missing empty state")
	}
}

func TestDashboardListsMonitorsWithUptime(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	m := addMonitor(t, st, "checkout")
	addMonitor(t, st, "search")
	ctx := context.Background()
	for i := range 4 {
		r := monitor.Result{MonitorID: m.ID, CheckedAt: time.Now().Add(-time.Duration(i) * time.Minute), Up: i != 0}
		if err := st.RecordCheck(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	c, _ := login(t, srv)
	body := getWith(t, srv, "/admin", c).Body.String()
	for _, want := range []string{"checkout", "search", "75.00%", "no data", "1m"} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard is missing %q", want)
		}
	}
}
