package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/sorin-work/status-page/internal/notify"
	"github.com/sorin-work/status-page/internal/store"
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

type fakeNotifier struct {
	delivered []notify.Event
	published []notify.Event
}

func (f *fakeNotifier) Publish(_ context.Context, ev notify.Event) {
	f.published = append(f.published, ev)
}

func (f *fakeNotifier) Deliver(_ context.Context, hook store.Webhook, ev notify.Event) store.Delivery {
	f.delivered = append(f.delivered, ev)
	return store.Delivery{WebhookID: hook.ID, Event: ev.Type, StatusCode: http.StatusOK}
}

func TestWebhookTestDelivery(t *testing.T) {
	srv, st := newTestServer(t)
	fn := &fakeNotifier{}
	srv.notifier = fn
	createAdmin(t, st)
	c, csrf := login(t, srv)
	hook, _ := st.CreateWebhook(context.Background(), store.Webhook{Name: "x", URL: "https://x.test", Secret: "k", Enabled: true})

	rec := postForm(t, srv, "/admin/webhooks/"+itoa(hook.ID)+"/test", url.Values{"csrf": {csrf}}, c)
	if rec.Code != http.StatusSeeOther || len(fn.delivered) != 1 || fn.delivered[0].Type != notify.EventTest {
		t.Errorf("test delivery: %d %+v", rec.Code, fn.delivered)
	}
}

func TestManualIncidentsPublishEvents(t *testing.T) {
	srv, st := newTestServer(t)
	fn := &fakeNotifier{}
	srv.notifier = fn
	createAdmin(t, st)
	c, csrf := login(t, srv)

	rec := postForm(t, srv, "/admin/incidents", url.Values{
		"csrf": {csrf}, "title": {"Login issues"}, "status": {"investigating"}, "impact": {"minor"}, "message": {"Looking."},
	}, c)
	postForm(t, srv, rec.Header().Get("Location")+"/updates", url.Values{"csrf": {csrf}, "status": {"resolved"}, "message": {"Fixed."}}, c)

	if len(fn.published) != 2 || fn.published[0].Type != notify.EventIncidentCreated || fn.published[1].Incident.Status != "resolved" {
		t.Fatalf("published = %+v", fn.published)
	}
	if fn.published[1].Incident.Message != "Fixed." || !strings.HasPrefix(fn.published[1].Incident.URL, "/incidents/") {
		t.Errorf("payload = %+v", fn.published[1].Incident)
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
