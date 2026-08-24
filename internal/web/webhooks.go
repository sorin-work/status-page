package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/sorin-work/status-page/internal/auth"
	"github.com/sorin-work/status-page/internal/incident"
	"github.com/sorin-work/status-page/internal/notify"
	"github.com/sorin-work/status-page/internal/store"
)

type webhookRow struct {
	Webhook store.Webhook
	Last    *store.Delivery
}

type webhooksData struct {
	Rows  []webhookRow
	Name  string
	URL   string
	Error string
}

func (s *Server) renderWebhooks(w http.ResponseWriter, r *http.Request, status int, data webhooksData) {
	hooks, err := s.store.ListWebhooks(r.Context(), false)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	for _, h := range hooks {
		row := webhookRow{Webhook: h}
		last, err := s.store.ListDeliveries(r.Context(), h.ID, 1)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		if len(last) > 0 {
			row.Last = &last[0]
		}
		data.Rows = append(data.Rows, row)
	}
	s.render(w, r, status, "webhooks", page{Title: "Webhooks", Nav: "webhooks", Data: data})
}

func (s *Server) handleWebhooks(w http.ResponseWriter, r *http.Request) {
	s.renderWebhooks(w, r, http.StatusOK, webhooksData{})
}

func validateWebhook(name, rawURL string) string {
	if name == "" {
		return "Name is required."
	}
	if utf8.RuneCountInString(name) > 100 {
		return "Name must be at most 100 characters."
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "URL must be an http:// or https:// address."
	}
	return ""
}

func (s *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PostFormValue("name"))
	rawURL := strings.TrimSpace(r.PostFormValue("url"))
	if msg := validateWebhook(name, rawURL); msg != "" {
		s.renderWebhooks(w, r, http.StatusUnprocessableEntity, webhooksData{Name: name, URL: rawURL, Error: msg})
		return
	}
	secret, err := auth.NewToken()
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if _, err := s.store.CreateWebhook(r.Context(), store.Webhook{Name: name, URL: rawURL, Secret: secret, Enabled: true}); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/webhooks", http.StatusSeeOther)
}

func (s *Server) loadWebhook(w http.ResponseWriter, r *http.Request) (store.Webhook, bool) {
	id, ok := pathID(r)
	if !ok {
		s.notFound(w, r)
		return store.Webhook{}, false
	}
	hook, err := s.store.GetWebhook(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		s.notFound(w, r)
		return store.Webhook{}, false
	}
	if err != nil {
		s.serverError(w, r, err)
		return store.Webhook{}, false
	}
	return hook, true
}

func (s *Server) handleToggleWebhook(w http.ResponseWriter, r *http.Request) {
	hook, ok := s.loadWebhook(w, r)
	if !ok {
		return
	}
	if err := s.store.SetWebhookEnabled(r.Context(), hook.ID, !hook.Enabled); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/webhooks", http.StatusSeeOther)
}

func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	hook, ok := s.loadWebhook(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteWebhook(r.Context(), hook.ID); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/webhooks", http.StatusSeeOther)
}

// Notifier delivers webhook events.
type Notifier interface {
	Publish(ctx context.Context, ev notify.Event)
	Deliver(ctx context.Context, hook store.Webhook, ev notify.Event) store.Delivery
}

func (s *Server) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	hook, ok := s.loadWebhook(w, r)
	if !ok {
		return
	}
	if s.notifier == nil {
		http.Error(w, "Webhooks are not configured.", http.StatusServiceUnavailable)
		return
	}
	res := s.notifier.Deliver(r.Context(), hook, notify.Event{
		Type:      notify.EventTest,
		Timestamp: s.now().UTC(),
		Incident:  &notify.IncidentInfo{Title: "Test delivery from " + s.site, Status: "resolved", Impact: "none", Message: "If you can read this, the webhook works."},
	})
	msg := fmt.Sprintf("Test delivered to %s (HTTP %d).", hook.Name, res.StatusCode)
	if !res.OK() {
		msg = fmt.Sprintf("Test delivery to %s failed: %s", hook.Name, res.Error)
	}
	s.redirectWithFlash(w, r, "/admin/webhooks/"+itoa64(hook.ID), msg)
}

// publishIncident sends an incident event if webhooks are configured.
func (s *Server) publishIncident(ctx context.Context, typ string, inc incident.Incident, status incident.Status, message string) {
	if s.notifier == nil {
		return
	}
	s.notifier.Publish(ctx, notify.Event{
		Type:      typ,
		Timestamp: s.now().UTC(),
		Incident: &notify.IncidentInfo{
			ID: inc.ID, Title: inc.Title, Status: string(status), Impact: string(inc.Impact),
			Message: strings.TrimSpace(message), URL: "/incidents/" + itoa64(inc.ID),
		},
	})
}

func (s *Server) handleWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	hook, ok := s.loadWebhook(w, r)
	if !ok {
		return
	}
	deliveries, err := s.store.ListDeliveries(r.Context(), hook.ID, 50)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "webhook_deliveries", page{Title: hook.Name, Nav: "webhooks", Data: struct {
		Webhook    store.Webhook
		Deliveries []store.Delivery
	}{hook, deliveries}})
}
