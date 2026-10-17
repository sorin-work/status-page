package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/sorin-work/status-page/internal/incident"
	"github.com/sorin-work/status-page/internal/store"
)

// Every state-changing admin route must refuse a request without the CSRF
// token, even from a logged in admin.
func TestAdminFormsRequireCSRF(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	ctx := context.Background()
	m := addMonitor(t, st, "api")
	comp, _ := st.CreateComponent(ctx, store.Component{Name: "API"})
	inc, _ := st.CreateIncident(ctx, incident.Incident{Title: "x", Status: incident.StatusInvestigating, Impact: incident.ImpactMinor}, "x")
	hook, _ := st.CreateWebhook(ctx, store.Webhook{Name: "x", URL: "https://x.test", Secret: "k", Enabled: true})
	c, _ := login(t, srv)

	paths := []string{
		"/admin/logout",
		"/admin/monitors",
		"/admin/monitors/" + itoa(m.ID),
		"/admin/monitors/" + itoa(m.ID) + "/delete",
		"/admin/monitors/" + itoa(m.ID) + "/pause",
		"/admin/monitors/" + itoa(m.ID) + "/resume",
		"/admin/components",
		"/admin/components/" + itoa(comp.ID),
		"/admin/components/" + itoa(comp.ID) + "/delete",
		"/admin/maintenance",
		"/admin/maintenance/1/end",
		"/admin/maintenance/1/delete",
		"/admin/account/password",
		"/admin/webhooks",
		"/admin/webhooks/" + itoa(hook.ID) + "/toggle",
		"/admin/webhooks/" + itoa(hook.ID) + "/delete",
		"/admin/webhooks/" + itoa(hook.ID) + "/test",
		"/admin/incidents",
		"/admin/incidents/" + itoa(inc.ID) + "/updates",
	}
	for _, p := range paths {
		if rec := postForm(t, srv, p, url.Values{"name": {"x"}}, c); rec.Code != http.StatusForbidden {
			t.Errorf("POST %s without CSRF token: %d", p, rec.Code)
		}
	}
	if _, err := st.GetMonitor(ctx, m.ID); err != nil {
		t.Error("a forged request deleted the monitor")
	}
}

// Every API route except the public status must refuse a request without
// the token.
func TestAPIRoutesRequireToken(t *testing.T) {
	srv, _ := newAPIServer(t)
	routes := []string{
		"GET /api/monitors", "POST /api/monitors", "GET /api/monitors/1", "PUT /api/monitors/1",
		"DELETE /api/monitors/1", "POST /api/monitors/1/pause", "POST /api/monitors/1/resume",
		"GET /api/monitors/1/checks", "GET /api/monitors/1/uptime", "GET /api/components",
		"GET /api/maintenance", "POST /api/maintenance", "DELETE /api/maintenance/1",
		"GET /api/incidents", "POST /api/incidents", "GET /api/incidents/1", "POST /api/incidents/1/updates",
	}
	for _, route := range routes {
		method, path, _ := strings.Cut(route, " ")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader("{}")))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without token: %d", route, rec.Code)
		}
	}
	if rec := get(t, srv, "/api/status"); rec.Code != http.StatusOK {
		t.Errorf("public status: %d", rec.Code)
	}
}
