package web

import (
	"context"
	"sync"
	"time"
)

// statusCache keeps the last status page data for a short time. The page
// is public and may be hit hard during an outage, which is exactly when
// the database is busiest.
type statusCache struct {
	ttl time.Duration
	now func() time.Time

	mu      sync.Mutex
	data    statusPage
	expires time.Time
}

func (c *statusCache) get(ctx context.Context, build func(context.Context) (statusPage, error)) (statusPage, error) {
	if c == nil || c.ttl <= 0 {
		return build(ctx)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.now().Before(c.expires) {
		return c.data, nil
	}
	data, err := build(ctx)
	if err != nil {
		return statusPage{}, err
	}
	c.data, c.expires = data, c.now().Add(c.ttl)
	return data, nil
}

// invalidate drops the cached data, e.g. after an admin change.
func (c *statusCache) invalidate() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.expires = time.Time{}
	c.mu.Unlock()
}
