package web

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/store"
)

func TestAPIComponents(t *testing.T) {
	srv, st := newAPIServer(t)
	if _, err := st.CreateComponent(context.Background(), store.Component{Name: "API", Description: "REST"}); err != nil {
		t.Fatal(err)
	}
	body := decode[struct{ Components []apiComponent }](t, apiCall(t, srv, http.MethodGet, "/api/components", nil))
	if len(body.Components) != 1 || body.Components[0].Description != "REST" {
		t.Errorf("components = %+v", body)
	}
}

func TestAPIMaintenance(t *testing.T) {
	srv, _ := newAPIServer(t)
	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	rec := apiCall(t, srv, http.MethodPost, "/api/maintenance", map[string]any{
		"title": "Kernel updates", "starts_at": start, "ends_at": start.Add(30 * time.Minute),
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	created := decode[apiMaintenance](t, rec)
	if !created.StartsAt.Equal(start) || created.Active {
		t.Errorf("created = %+v", created)
	}
	list := decode[struct{ Maintenance []apiMaintenance }](t, apiCall(t, srv, http.MethodGet, "/api/maintenance", nil))
	if len(list.Maintenance) != 1 {
		t.Errorf("list = %+v", list)
	}

	rec = apiCall(t, srv, http.MethodPost, "/api/maintenance", map[string]any{"title": "x", "starts_at": start, "ends_at": start.Add(-time.Hour), "component_id": 9})
	if body := decode[apiError](t, rec); rec.Code != http.StatusUnprocessableEntity || body.Fields["ends_at"] == "" || body.Fields["component_id"] == "" {
		t.Errorf("validation: %d %+v", rec.Code, body)
	}

	path := "/api/maintenance/" + itoa(created.ID)
	if rec := apiCall(t, srv, http.MethodDelete, path, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec := apiCall(t, srv, http.MethodDelete, path, nil); rec.Code != http.StatusNotFound {
		t.Errorf("second delete: %d", rec.Code)
	}
}
