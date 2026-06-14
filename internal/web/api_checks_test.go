package web

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
)

func TestAPIListChecks(t *testing.T) {
	srv, st := newAPIServer(t)
	m := addMonitor(t, st, "api")
	start := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	for i := range 5 {
		r := monitor.Result{MonitorID: m.ID, CheckedAt: start.Add(time.Duration(i) * time.Minute), Up: i != 2, Latency: 50 * time.Millisecond, StatusCode: 200}
		if i == 2 {
			r.Error, r.StatusCode = "timed out after 10s", 0
		}
		if err := st.RecordCheck(context.Background(), r); err != nil {
			t.Fatal(err)
		}
	}
	rec := apiCall(t, srv, http.MethodGet, "/api/monitors/"+itoa(m.ID)+"/checks?limit=2&offset=2", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	body := decode[struct {
		Checks []apiCheck
		Total  int
	}](t, rec)
	if body.Total != 5 || len(body.Checks) != 2 || body.Checks[0].Up || body.Checks[0].Error == "" || body.Checks[1].LatencyMS != 50 {
		t.Errorf("body = %+v", body)
	}
	for _, q := range []string{"?limit=0", "?limit=5000", "?offset=-1", "?limit=abc"} {
		if rec := apiCall(t, srv, http.MethodGet, "/api/monitors/"+itoa(m.ID)+"/checks"+q, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", q, rec.Code)
		}
	}
}
