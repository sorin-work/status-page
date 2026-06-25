package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	body := []byte(`{"event":"test"}`)
	sig := Sign("s3cret", body)
	if !strings.HasPrefix(sig, "sha256=") || len(sig) != len("sha256=")+64 {
		t.Fatalf("unexpected signature %q", sig)
	}
	if !Verify("s3cret", body, sig) {
		t.Error("valid signature rejected")
	}
	if Verify("other", body, sig) || Verify("s3cret", []byte(`{}`), sig) {
		t.Error("forged signature accepted")
	}
}

func TestNewRequest(t *testing.T) {
	ev := Event{
		Type:      EventMonitorDown,
		Timestamp: time.Date(2026, 6, 25, 18, 0, 0, 0, time.UTC),
		Monitor:   &MonitorInfo{ID: 3, Name: "Checkout", Target: "https://shop.test/checkout", Status: "down", Error: "timeout"},
	}
	req, err := newRequest(context.Background(), "https://hooks.test/in", "key", "d-1", ev)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != http.MethodPost || req.Header.Get(HeaderEvent) != EventMonitorDown || req.Header.Get(HeaderDelivery) != "d-1" {
		t.Errorf("headers = %v", req.Header)
	}
	body, _ := io.ReadAll(req.Body)
	if !Verify("key", body, req.Header.Get(HeaderSignature)) {
		t.Error("signature does not match the body")
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["event"] != "monitor.down" || got["incident"] != nil || got["monitor"].(map[string]any)["name"] != "Checkout" {
		t.Errorf("body = %s", body)
	}
}
