package web

import (
	"net/http"
	"testing"
)

func TestAPIIncidentLifecycle(t *testing.T) {
	srv, _ := newAPIServer(t)

	rec := apiCall(t, srv, http.MethodPost, "/api/incidents", map[string]any{
		"title": "Webhooks delayed", "impact": "minor", "message": "Deliveries are queued.",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	inc := decode[apiIncident](t, rec)
	if inc.Status != "investigating" || inc.ResolvedAt != nil || len(inc.Updates) != 1 {
		t.Errorf("created = %+v", inc)
	}
	path := "/api/incidents/" + itoa(inc.ID)

	rec = apiCall(t, srv, http.MethodPost, path+"/updates", map[string]any{"status": "resolved", "message": "Queue drained."})
	if rec.Code != http.StatusCreated {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	got := decode[apiIncident](t, apiCall(t, srv, http.MethodGet, path, nil))
	if got.Status != "resolved" || got.ResolvedAt == nil || len(got.Updates) != 2 || got.Updates[0].Message != "Queue drained." {
		t.Errorf("after update = %+v", got)
	}

	list := decode[struct{ Incidents []apiIncident }](t, apiCall(t, srv, http.MethodGet, "/api/incidents", nil))
	if len(list.Incidents) != 1 {
		t.Errorf("list = %+v", list)
	}
}

func TestAPIIncidentValidation(t *testing.T) {
	srv, _ := newAPIServer(t)
	rec := apiCall(t, srv, http.MethodPost, "/api/incidents", map[string]any{"title": "", "impact": "apocalyptic", "component_id": 5})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d", rec.Code)
	}
	body := decode[apiError](t, rec)
	for _, f := range []string{"title", "impact", "message", "component_id"} {
		if body.Fields[f] == "" {
			t.Errorf("no error for %s", f)
		}
	}
	if rec := apiCall(t, srv, http.MethodPost, "/api/incidents/42/updates", map[string]any{"message": "x"}); rec.Code != http.StatusNotFound {
		t.Errorf("update on missing incident: %d", rec.Code)
	}
	if rec := apiCall(t, srv, http.MethodGet, "/api/incidents?limit=900", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("limit too big: %d", rec.Code)
	}
}
