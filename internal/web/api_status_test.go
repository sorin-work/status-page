package web

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/incident"
	"github.com/sorin-work/status-page/internal/maintenance"
	"github.com/sorin-work/status-page/internal/store"
)

func TestAPIStatusIsPublic(t *testing.T) {
	srv, st := newAPIServer(t)
	ctx := context.Background()
	comp, _ := st.CreateComponent(ctx, store.Component{Name: "Checkout"})
	if _, err := st.CreateIncident(ctx, incident.Incident{Title: "Payments failing", Status: incident.StatusIdentified, Impact: incident.ImpactMajor, ComponentID: comp.ID}, "Provider outage."); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateMaintenance(ctx, maintenance.Window{Title: "DB upgrade", StartsAt: time.Now().Add(time.Hour), EndsAt: time.Now().Add(2 * time.Hour)}); err != nil {
		t.Fatal(err)
	}

	rec := get(t, srv, "/api/status")
	if rec.Code != http.StatusOK || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("got %d", rec.Code)
	}
	body := decode[struct {
		Status      string
		Components  []apiComponentStatus
		Incidents   []apiIncident
		Maintenance []apiMaintenance
	}](t, rec)
	if body.Status != "major_outage" || len(body.Components) != 1 || body.Components[0].Status != "major_outage" {
		t.Errorf("status = %+v", body)
	}
	if len(body.Incidents) != 1 || len(body.Maintenance) != 1 || body.Maintenance[0].Active {
		t.Errorf("incidents/maintenance = %+v / %+v", body.Incidents, body.Maintenance)
	}
}
