package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWebhooks(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	a, err := s.CreateWebhook(ctx, Webhook{Name: "chat", URL: "https://hooks.example.com/a", Secret: "s1", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.CreateWebhook(ctx, Webhook{Name: "pager", URL: "https://hooks.example.com/b", Secret: "s2", Enabled: true})

	if err := s.SetWebhookEnabled(ctx, b.ID, false); err != nil {
		t.Fatal(err)
	}
	enabled, _ := s.ListWebhooks(ctx, true)
	all, _ := s.ListWebhooks(ctx, false)
	if len(enabled) != 1 || enabled[0].ID != a.ID || len(all) != 2 {
		t.Errorf("enabled=%+v all=%+v", enabled, all)
	}
	got, err := s.GetWebhook(ctx, a.ID)
	if err != nil || got.Secret != "s1" {
		t.Errorf("get = %+v %v", got, err)
	}

	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	for i, d := range []Delivery{
		{WebhookID: a.ID, Event: "monitor.down", Attempt: 1, Error: "connection refused", CreatedAt: now},
		{WebhookID: a.ID, Event: "monitor.down", Attempt: 2, StatusCode: 200, Duration: 120 * time.Millisecond, CreatedAt: now.Add(time.Minute)},
	} {
		if err := s.RecordDelivery(ctx, d); err != nil {
			t.Fatalf("delivery %d: %v", i, err)
		}
	}
	log, err := s.ListDeliveries(ctx, a.ID, 10)
	if err != nil || len(log) != 2 || !log[0].OK() || log[1].OK() || log[0].Duration != 120*time.Millisecond {
		t.Errorf("deliveries = %+v %v", log, err)
	}

	if err := s.DeleteWebhook(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetWebhook(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted webhook found: %v", err)
	}
	if log, _ := s.ListDeliveries(ctx, a.ID, 10); len(log) != 0 {
		t.Errorf("deliveries of deleted webhook remain")
	}
}
