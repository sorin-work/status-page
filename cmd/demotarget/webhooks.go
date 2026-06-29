package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// received is a webhook delivery the demo target got from status-page.
type received struct {
	ReceivedAt     time.Time       `json:"received_at"`
	Event          string          `json:"event"`
	Delivery       string          `json:"delivery"`
	SignatureValid *bool           `json:"signature_valid,omitempty"`
	Body           json.RawMessage `json:"body"`
}

const maxReceived = 50

// receiveWebhook accepts deliveries so the demo setup can show webhooks
// end to end. With WEBHOOK_SECRET set it also checks the signature.
func (t *target) receiveWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || !json.Valid(body) {
		http.Error(w, "expected a JSON body", http.StatusBadRequest)
		return
	}
	rec := received{
		ReceivedAt: time.Now().UTC(),
		Event:      r.Header.Get("X-StatusPage-Event"),
		Delivery:   r.Header.Get("X-StatusPage-Delivery"),
		Body:       body,
	}
	if t.webhookSecret != "" {
		mac := hmac.New(sha256.New, []byte(t.webhookSecret))
		mac.Write(body)
		valid := hmac.Equal([]byte("sha256="+hex.EncodeToString(mac.Sum(nil))), []byte(r.Header.Get("X-StatusPage-Signature")))
		rec.SignatureValid = &valid
	}
	t.mu.Lock()
	t.received = append([]received{rec}, t.received...)
	if len(t.received) > maxReceived {
		t.received = t.received[:maxReceived]
	}
	t.mu.Unlock()
	t.logger.Info("webhook received", "event", rec.Event, "delivery", rec.Delivery)
	w.WriteHeader(http.StatusNoContent)
}

func (t *target) listWebhooks(w http.ResponseWriter, _ *http.Request) {
	t.mu.Lock()
	list := append([]received{}, t.received...)
	t.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"received": list})
}
