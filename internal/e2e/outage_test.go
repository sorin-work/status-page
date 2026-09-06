// Package e2e runs the scheduler, alerting, webhooks and the web server
// together against real HTTP targets.
package e2e

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sorin-work/status-page/internal/alerting"
	"github.com/sorin-work/status-page/internal/monitor"
	"github.com/sorin-work/status-page/internal/notify"
	"github.com/sorin-work/status-page/internal/scheduler"
	"github.com/sorin-work/status-page/internal/store"
	"github.com/sorin-work/status-page/internal/web"
)

type hookReceiver struct {
	mu     sync.Mutex
	events []string
}

func (h *hookReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	if !notify.Verify("e2e-secret", body, r.Header.Get(notify.HeaderSignature)) {
		http.Error(w, "bad signature", http.StatusUnauthorized)
		return
	}
	var ev struct{ Event string }
	_ = json.Unmarshal(body, &ev)
	h.mu.Lock()
	h.events = append(h.events, ev.Event)
	h.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (h *hookReceiver) list() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.events...)
}

func TestOutageEndToEnd(t *testing.T) {
	ctx := context.Background()
	quiet := slog.New(slog.DiscardHandler)

	var down atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if down.Load() {
			http.Error(w, "maintenance", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("shop is open"))
	}))
	defer target.Close()
	receiver := &hookReceiver{}
	hooks := httptest.NewServer(receiver)
	defer hooks.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "e2e.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	comp, _ := st.CreateComponent(ctx, store.Component{Name: "Shop"})
	if _, err := st.CreateMonitor(ctx, monitor.Monitor{
		Name: "Shop", Type: monitor.TypeHTTP, Target: target.URL, Interval: time.Minute, Timeout: 2 * time.Second,
		Keyword: "open", Retries: 1, ComponentID: comp.ID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateWebhook(ctx, store.Webhook{Name: "test", URL: hooks.URL, Secret: "e2e-secret", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	dispatcher := notify.NewDispatcher(st, quiet)
	dispatcher.Start(1)
	alerts := alerting.New(st, quiet)
	alerts.Publisher = dispatcher
	sched := scheduler.New(st, monitor.NewChecker(), 2, quiet)
	sched.OnTransition = alerts.HandleTransition
	clock := time.Now()
	sched.SetClock(func() time.Time { return clock })
	step := func() {
		sched.RunDue(ctx)
		sched.Wait()
		clock = clock.Add(time.Minute)
	}

	srv, err := web.New(web.Options{Store: st, Logger: quiet, SiteTitle: "E2E"})
	if err != nil {
		t.Fatal(err)
	}
	page := func() string {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		return rec.Body.String()
	}

	step()
	if !strings.Contains(page(), "All systems operational") {
		t.Fatal("status page not operational after the first check")
	}

	down.Store(true)
	step() // first failure: still up thanks to the retry
	if strings.Contains(page(), "Major outage") {
		t.Fatal("outage reported before the retry was used")
	}
	step() // second failure: down
	body := page()
	if !strings.Contains(body, "Major outage") || !strings.Contains(body, "Shop is down") {
		t.Fatal("outage not shown on the status page")
	}

	down.Store(false)
	step()
	body = page()
	if !strings.Contains(body, "All systems operational") || !strings.Contains(body, "Shop is responding again") {
		t.Fatal("recovery not shown on the status page")
	}

	dispatcher.Close()
	want := []string{"monitor.down", "incident.created", "monitor.up", "incident.updated"}
	got := receiver.list()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("webhooks = %v, want %v", got, want)
	}
}
