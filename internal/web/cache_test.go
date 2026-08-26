package web

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStatusCache(t *testing.T) {
	now := time.Date(2026, 8, 26, 18, 0, 0, 0, time.UTC)
	c := &statusCache{ttl: 5 * time.Second, now: func() time.Time { return now }}
	calls := 0
	build := func(context.Context) (statusPage, error) {
		calls++
		return statusPage{Headline: "ok"}, nil
	}
	ctx := context.Background()
	for range 3 {
		if _, err := c.get(ctx, build); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Errorf("built %d times within the TTL", calls)
	}
	now = now.Add(6 * time.Second)
	_, _ = c.get(ctx, build)
	c.invalidate()
	_, _ = c.get(ctx, build)
	if calls != 3 {
		t.Errorf("built %d times, want 3", calls)
	}

	failing := func(context.Context) (statusPage, error) { return statusPage{}, errors.New("db down") }
	c.invalidate()
	if _, err := c.get(ctx, failing); err == nil {
		t.Error("error not returned")
	}
	if _, err := c.get(ctx, build); err != nil || calls != 4 {
		t.Error("a failed build should not be cached")
	}
}
