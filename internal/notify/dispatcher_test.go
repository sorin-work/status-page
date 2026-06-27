package notify

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "notify.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return st
}

type receiver struct {
	mu     sync.Mutex
	bodies [][]byte
	sigs   []string
	status int
}

func (r *receiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	r.bodies = append(r.bodies, body)
	r.sigs = append(r.sigs, req.Header.Get(HeaderSignature))
	status := r.status
	r.mu.Unlock()
	if status == 0 {
		status = http.StatusNoContent
	}
	w.WriteHeader(status)
}

func (r *receiver) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.bodies)
}

func TestPublishDeliversToEnabledWebhooks(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	rcv := &receiver{}
	srv := httptest.NewServer(rcv)
	defer srv.Close()

	on, _ := st.CreateWebhook(ctx, store.Webhook{Name: "on", URL: srv.URL, Secret: "k1", Enabled: true})
	if _, err := st.CreateWebhook(ctx, store.Webhook{Name: "off", URL: srv.URL, Secret: "k2", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	d := NewDispatcher(st, slog.New(slog.DiscardHandler))
	d.Start(2)
	d.Publish(ctx, Event{Type: EventTest})
	d.Close()

	if rcv.count() != 1 {
		t.Fatalf("%d deliveries, want 1", rcv.count())
	}
	if !Verify("k1", rcv.bodies[0], rcv.sigs[0]) {
		t.Error("bad signature")
	}
	log, _ := st.ListDeliveries(ctx, on.ID, 10)
	if len(log) != 1 || !log[0].OK() || log[0].StatusCode != http.StatusNoContent {
		t.Errorf("delivery log = %+v", log)
	}
}

func TestDeliverRecordsFailures(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	rcv := &receiver{status: http.StatusInternalServerError}
	srv := httptest.NewServer(rcv)
	defer srv.Close()
	hook, _ := st.CreateWebhook(ctx, store.Webhook{Name: "x", URL: srv.URL, Secret: "k", Enabled: true})

	d := NewDispatcher(st, slog.New(slog.DiscardHandler))
	res := d.Deliver(ctx, hook, Event{Type: EventTest, Timestamp: time.Now()})
	if res.OK() || res.StatusCode != http.StatusInternalServerError || res.Error == "" {
		t.Errorf("result = %+v", res)
	}

	srv.Close()
	res = d.Deliver(ctx, hook, Event{Type: EventTest})
	if res.OK() || res.StatusCode != 0 || res.Error == "" {
		t.Errorf("unreachable receiver: %+v", res)
	}
	if log, _ := st.ListDeliveries(ctx, hook.ID, 10); len(log) != 2 {
		t.Errorf("%d deliveries logged", len(log))
	}
}

// flaky fails the first n requests with 503.
type flaky struct {
	mu    sync.Mutex
	fails int
	ids   []string
}

func (f *flaky) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ids = append(f.ids, r.Header.Get(HeaderDelivery))
	if f.fails > 0 {
		f.fails--
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func TestRetriesUntilSuccess(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	rcv := &flaky{fails: 2}
	srv := httptest.NewServer(rcv)
	defer srv.Close()
	hook, _ := st.CreateWebhook(ctx, store.Webhook{Name: "x", URL: srv.URL, Secret: "k", Enabled: true})

	d := NewDispatcher(st, slog.New(slog.DiscardHandler))
	d.Backoff = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	d.Start(1)
	d.Publish(ctx, Event{Type: EventTest})
	d.Close()

	log, _ := st.ListDeliveries(ctx, hook.ID, 10)
	if len(log) != 3 || !log[0].OK() || log[0].Attempt != 3 || log[2].Attempt != 1 {
		t.Fatalf("deliveries = %+v", log)
	}
	if rcv.ids[0] != rcv.ids[2] {
		t.Errorf("retries used different delivery ids: %v", rcv.ids)
	}
}

func TestGivesUpAfterBackoff(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	srv := httptest.NewServer(&flaky{fails: 100})
	defer srv.Close()
	hook, _ := st.CreateWebhook(ctx, store.Webhook{Name: "x", URL: srv.URL, Secret: "k", Enabled: true})

	d := NewDispatcher(st, slog.New(slog.DiscardHandler))
	d.Backoff = []time.Duration{time.Millisecond}
	d.Start(1)
	d.Publish(ctx, Event{Type: EventTest})
	d.Close()
	if log, _ := st.ListDeliveries(ctx, hook.ID, 10); len(log) != 2 {
		t.Errorf("%d attempts, want 2", len(log))
	}
}

func TestNoRetryOnClientError(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	srv := httptest.NewServer(&receiver{status: http.StatusGone})
	defer srv.Close()
	hook, _ := st.CreateWebhook(ctx, store.Webhook{Name: "x", URL: srv.URL, Secret: "k", Enabled: true})

	d := NewDispatcher(st, slog.New(slog.DiscardHandler))
	d.Backoff = []time.Duration{time.Millisecond, time.Millisecond}
	d.Start(1)
	d.Publish(ctx, Event{Type: EventTest})
	d.Close()
	if log, _ := st.ListDeliveries(ctx, hook.ID, 10); len(log) != 1 {
		t.Errorf("%d attempts, want 1", len(log))
	}
}
