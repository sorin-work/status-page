package notify

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/sorin-work/status-page/internal/auth"
	"github.com/sorin-work/status-page/internal/store"
)

// Store is the persistence the dispatcher needs.
type Store interface {
	ListWebhooks(ctx context.Context, enabledOnly bool) ([]store.Webhook, error)
	RecordDelivery(ctx context.Context, d store.Delivery) error
}

type job struct {
	hook store.Webhook
	ev   Event
}

// Dispatcher sends events to every enabled webhook in the background.
type Dispatcher struct {
	store  Store
	client *http.Client
	logger *slog.Logger
	queue  chan job
	wg     sync.WaitGroup

	// Backoff is the wait before each retry of a failed delivery. Its
	// length is the number of retries.
	Backoff []time.Duration
}

// QueueSize is how many deliveries may wait before new ones are dropped.
const QueueSize = 256

// NewDispatcher returns a dispatcher. Call Start before Publish.
func NewDispatcher(st Store, logger *slog.Logger) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Dispatcher{
		store:  st,
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
		queue:  make(chan job, QueueSize),

		Backoff: []time.Duration{5 * time.Second, 30 * time.Second, 2 * time.Minute},
	}
}

// Start runs the delivery workers until Close is called.
func (d *Dispatcher) Start(workers int) {
	for range max(workers, 1) {
		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			for j := range d.queue {
				d.deliverWithRetries(j.hook, j.ev)
			}
		}()
	}
}

// Close stops accepting events and waits for queued deliveries.
func (d *Dispatcher) Close() {
	close(d.queue)
	d.wg.Wait()
}

// Publish queues ev for every enabled webhook. It never blocks: when the
// queue is full the delivery is dropped and logged.
func (d *Dispatcher) Publish(ctx context.Context, ev Event) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now().UTC()
	}
	hooks, err := d.store.ListWebhooks(ctx, true)
	if err != nil {
		d.logger.Error("list webhooks", "err", err)
		return
	}
	for _, h := range hooks {
		select {
		case d.queue <- job{hook: h, ev: ev}:
		default:
			d.logger.Warn("webhook queue full, dropping delivery", "webhook", h.ID, "event", ev.Type)
		}
	}
}

// deliverWithRetries tries a delivery until it succeeds, fails for good or
// runs out of retries. Every attempt carries the same delivery ID so
// receivers can deduplicate.
func (d *Dispatcher) deliverWithRetries(hook store.Webhook, ev Event) {
	id := deliveryID()
	for attempt := 1; ; attempt++ {
		res := d.attempt(context.Background(), hook, ev, id, attempt)
		if res.OK() || !retryable(res) || attempt > len(d.Backoff) {
			return
		}
		time.Sleep(d.Backoff[attempt-1])
	}
}

// retryable reports whether a failed delivery may succeed later: network
// errors, rate limiting and server errors are, other 4xx answers are not.
func retryable(d store.Delivery) bool {
	switch {
	case d.StatusCode == 0:
		return true
	case d.StatusCode == http.StatusTooManyRequests, d.StatusCode >= 500:
		return true
	}
	return false
}

func deliveryID() string {
	id, err := auth.NewToken()
	if err != nil {
		return "unknown"
	}
	return id[:12]
}

// Deliver sends one event to one webhook right away, without retries, and
// logs the result.
func (d *Dispatcher) Deliver(ctx context.Context, hook store.Webhook, ev Event) store.Delivery {
	return d.attempt(ctx, hook, ev, deliveryID(), 1)
}

func (d *Dispatcher) attempt(ctx context.Context, hook store.Webhook, ev Event, id string, n int) store.Delivery {
	res := store.Delivery{WebhookID: hook.ID, Event: ev.Type, Attempt: n}
	start := time.Now()
	req, err := newRequest(ctx, hook.URL, hook.Secret, id, ev)
	if err != nil {
		res.Error = err.Error()
	} else {
		resp, err := d.client.Do(req)
		if err != nil {
			res.Error = err.Error()
		} else {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
			_ = resp.Body.Close()
			res.StatusCode = resp.StatusCode
			if !res.OK() {
				res.Error = "receiver answered " + resp.Status
			}
		}
	}
	res.Duration = time.Since(start)
	if err := d.store.RecordDelivery(ctx, res); err != nil {
		d.logger.Error("record webhook delivery", "webhook", hook.ID, "err", err)
	}
	if !res.OK() {
		d.logger.Warn("webhook delivery failed", "webhook", hook.ID, "event", ev.Type, "attempt", n, "error", res.Error)
	}
	return res
}
