package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestComponentsCRUD(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, csrf := login(t, srv)
	ctx := context.Background()

	if body := getWith(t, srv, "/admin/components", c).Body.String(); !strings.Contains(body, "No components yet") {
		t.Error("missing empty state")
	}
	rec := postForm(t, srv, "/admin/components", url.Values{"csrf": {csrf}, "name": {"API"}, "description": {"Public REST API"}}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d", rec.Code)
	}
	list, _ := st.ListComponents(ctx)
	if len(list) != 1 || list[0].Name != "API" {
		t.Fatalf("components = %+v", list)
	}
	id := itoa(list[0].ID)

	rec = postForm(t, srv, "/admin/components/"+id, url.Values{"csrf": {csrf}, "name": {"Public API"}, "description": {""}, "position": {"5"}}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update: %d", rec.Code)
	}
	got, _ := st.GetComponent(ctx, list[0].ID)
	if got.Name != "Public API" || got.Position != 5 {
		t.Errorf("updated = %+v", got)
	}

	rec = postForm(t, srv, "/admin/components/"+id+"/delete", url.Values{"csrf": {csrf}}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d", rec.Code)
	}
	if list, _ := st.ListComponents(ctx); len(list) != 0 {
		t.Error("component not deleted")
	}
}

func TestComponentValidation(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, csrf := login(t, srv)
	rec := postForm(t, srv, "/admin/components", url.Values{"csrf": {csrf}, "name": {" "}}, c)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Name is required") {
		t.Errorf("got %d", rec.Code)
	}
	if rec := postForm(t, srv, "/admin/components/42/delete", url.Values{"csrf": {csrf}}, c); rec.Code != http.StatusNotFound {
		t.Errorf("delete missing: %d", rec.Code)
	}
}
