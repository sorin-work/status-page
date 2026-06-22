package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Webhook is an endpoint that receives event notifications.
type Webhook struct {
	ID        int64
	Name      string
	URL       string
	Secret    string
	Enabled   bool
	CreatedAt time.Time
}

// Delivery is one attempt to send an event to a webhook.
type Delivery struct {
	ID         int64
	WebhookID  int64
	Event      string
	Attempt    int
	StatusCode int
	Error      string
	Duration   time.Duration
	CreatedAt  time.Time
}

// OK reports whether the receiver accepted the delivery.
func (d Delivery) OK() bool {
	return d.Error == "" && d.StatusCode >= 200 && d.StatusCode < 300
}

// CreateWebhook adds a webhook.
func (s *Store) CreateWebhook(ctx context.Context, w Webhook) (Webhook, error) {
	w.CreatedAt = s.now().UTC().Truncate(time.Second)
	res, err := s.db.ExecContext(ctx, `INSERT INTO webhooks (name, url, secret, enabled, created_at) VALUES (?, ?, ?, ?, ?)`,
		w.Name, w.URL, w.Secret, w.Enabled, w.CreatedAt.Unix())
	if err != nil {
		return Webhook{}, err
	}
	w.ID, err = res.LastInsertId()
	return w, err
}

// GetWebhook returns one webhook.
func (s *Store) GetWebhook(ctx context.Context, id int64) (Webhook, error) {
	var (
		w       Webhook
		created int64
	)
	err := s.db.QueryRowContext(ctx, `SELECT id, name, url, secret, enabled, created_at FROM webhooks WHERE id = ?`, id).
		Scan(&w.ID, &w.Name, &w.URL, &w.Secret, &w.Enabled, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Webhook{}, ErrNotFound
	}
	w.CreatedAt = time.Unix(created, 0).UTC()
	return w, err
}

// ListWebhooks returns every webhook, or only enabled ones.
func (s *Store) ListWebhooks(ctx context.Context, enabledOnly bool) ([]Webhook, error) {
	query := `SELECT id, name, url, secret, enabled, created_at FROM webhooks`
	if enabledOnly {
		query += ` WHERE enabled = 1`
	}
	rows, err := s.db.QueryContext(ctx, query+` ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Webhook
	for rows.Next() {
		var (
			w       Webhook
			created int64
		)
		if err := rows.Scan(&w.ID, &w.Name, &w.URL, &w.Secret, &w.Enabled, &created); err != nil {
			return nil, err
		}
		w.CreatedAt = time.Unix(created, 0).UTC()
		out = append(out, w)
	}
	return out, rows.Err()
}

// SetWebhookEnabled turns a webhook on or off.
func (s *Store) SetWebhookEnabled(ctx context.Context, id int64, enabled bool) error {
	res, err := s.db.ExecContext(ctx, `UPDATE webhooks SET enabled = ? WHERE id = ?`, enabled, id)
	if err != nil {
		return err
	}
	return expectOneRow(res)
}

// DeleteWebhook removes a webhook and its delivery log.
func (s *Store) DeleteWebhook(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return expectOneRow(res)
}

// RecordDelivery logs a delivery attempt.
func (s *Store) RecordDelivery(ctx context.Context, d Delivery) error {
	if d.CreatedAt.IsZero() {
		d.CreatedAt = s.now()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO webhook_deliveries
		(webhook_id, event, attempt, status_code, error, duration_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		d.WebhookID, d.Event, d.Attempt, d.StatusCode, d.Error, d.Duration.Milliseconds(), d.CreatedAt.Unix())
	return err
}

// ListDeliveries returns the latest delivery attempts of a webhook.
func (s *Store) ListDeliveries(ctx context.Context, webhookID int64, limit int) ([]Delivery, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, webhook_id, event, attempt, status_code, error, duration_ms, created_at
		FROM webhook_deliveries WHERE webhook_id = ? ORDER BY created_at DESC, id DESC LIMIT ?`, webhookID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Delivery
	for rows.Next() {
		var (
			d           Delivery
			durationMS  int64
			createdUnix int64
		)
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.Event, &d.Attempt, &d.StatusCode, &d.Error, &durationMS, &createdUnix); err != nil {
			return nil, err
		}
		d.Duration = time.Duration(durationMS) * time.Millisecond
		d.CreatedAt = time.Unix(createdUnix, 0).UTC()
		out = append(out, d)
	}
	return out, rows.Err()
}
