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
	}
}

// Start runs the delivery workers until Close is called.
func (d *Dispatcher) Start(workers int) {
	for range max(workers, 1) {
		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			for j := range d.queue {
				d.Deliver(context.Background(), j.hook, j.ev)
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

// Deliver sends one event to one webhook right away and logs the result.
func (d *Dispatcher) Deliver(ctx context.Context, hook store.Webhook, ev Event) store.Delivery {
	id, err := auth.NewToken()
	if err != nil {
		id = "unknown"
	}
	res := store.Delivery{WebhookID: hook.ID, Event: ev.Type, Attempt: 1}
	start := time.Now()
	req, err := newRequest(ctx, hook.URL, hook.Secret, id[:12], ev)
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
		d.logger.Warn("webhook delivery failed", "webhook", hook.ID, "event", ev.Type, "error", res.Error)
	}
	return res
}
