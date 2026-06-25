// Package notify delivers events to webhooks.
package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

// Event types.
const (
	EventMonitorDown     = "monitor.down"
	EventMonitorUp       = "monitor.up"
	EventIncidentCreated = "incident.created"
	EventIncidentUpdated = "incident.updated"
	EventTest            = "test"
)

// Headers sent with every delivery.
const (
	HeaderEvent     = "X-StatusPage-Event"
	HeaderSignature = "X-StatusPage-Signature"
	HeaderDelivery  = "X-StatusPage-Delivery"
)

// MonitorInfo describes the monitor an event is about.
type MonitorInfo struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Target string `json:"target"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// IncidentInfo describes the incident an event is about.
type IncidentInfo struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Impact  string `json:"impact"`
	Message string `json:"message,omitempty"`
	URL     string `json:"url,omitempty"`
}

// Event is the JSON body of a webhook delivery.
type Event struct {
	Type      string        `json:"event"`
	Timestamp time.Time     `json:"timestamp"`
	Monitor   *MonitorInfo  `json:"monitor,omitempty"`
	Incident  *IncidentInfo `json:"incident,omitempty"`
}

// Sign returns the signature header value for a body: the hex HMAC-SHA256
// of the body with the webhook secret, prefixed with "sha256=".
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Verify checks a signature header in constant time. Receivers can use it
// as a reference implementation.
func Verify(secret string, body []byte, signature string) bool {
	return hmac.Equal([]byte(Sign(secret, body)), []byte(signature))
}

// newRequest builds the HTTP request for one delivery.
func newRequest(ctx context.Context, url, secret, deliveryID string, ev Event) (*http.Request, error) {
	body, err := json.Marshal(ev)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "status-page-webhooks/1")
	req.Header.Set(HeaderEvent, ev.Type)
	req.Header.Set(HeaderDelivery, deliveryID)
	req.Header.Set(HeaderSignature, Sign(secret, body))
	return req, nil
}
