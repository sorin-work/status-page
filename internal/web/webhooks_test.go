package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestWebhookAdmin(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, csrf := login(t, srv)
	ctx := context.Background()

	if body := getWith(t, srv, "/admin/webhooks", c).Body.String(); !strings.Contains(body, "No webhooks yet") {
		t.Error("missing empty state")
	}
	rec := postForm(t, srv, "/admin/webhooks", url.Values{"csrf": {csrf}, "name": {"Chat"}, "url": {"https://hooks.example.com/x"}}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d", rec.Code)
	}
	hooks, _ := st.ListWebhooks(ctx, false)
	if len(hooks) != 1 || len(hooks[0].Secret) < 32 || !hooks[0].Enabled {
		t.Fatalf("hooks = %+v", hooks)
	}
	id := itoa(hooks[0].ID)

	postForm(t, srv, "/admin/webhooks/"+id+"/toggle", url.Values{"csrf": {csrf}}, c)
	if h, _ := st.GetWebhook(ctx, hooks[0].ID); h.Enabled {
		t.Error("webhook still enabled")
	}
	if rec := postForm(t, srv, "/admin/webhooks/"+id+"/delete", url.Values{"csrf": {csrf}}, c); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d", rec.Code)
	}
	if hooks, _ := st.ListWebhooks(ctx, false); len(hooks) != 0 {
		t.Error("webhook not deleted")
	}
}

func TestWebhookValidation(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, csrf := login(t, srv)
	for _, form := range []url.Values{
		{"csrf": {csrf}, "name": {""}, "url": {"https://x.test"}},
		{"csrf": {csrf}, "name": {"x"}, "url": {"ftp://x.test"}},
		{"csrf": {csrf}, "name": {"x"}, "url": {"hooks"}},
	} {
		if rec := postForm(t, srv, "/admin/webhooks", form, c); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%v: got %d", form, rec.Code)
		}
	}
}
