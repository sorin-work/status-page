package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSessions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }

	u, err := s.CreateUser(ctx, "admin", "hash")
	if err != nil {
		t.Fatal(err)
	}
	live := Session{TokenHash: "live", UserID: u.ID, CSRFToken: "csrf", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	old := Session{TokenHash: "old", UserID: u.ID, CSRFToken: "csrf2", CreatedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Hour)}
	for _, sess := range []Session{live, old} {
		if err := s.CreateSession(ctx, sess); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.GetSession(ctx, "live")
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "admin" || got.CSRFToken != "csrf" || !got.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Errorf("unexpected session: %+v", got)
	}
	if _, err := s.GetSession(ctx, "old"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired session returned: %v", err)
	}

	n, err := s.DeleteExpiredSessions(ctx)
	if err != nil || n != 1 {
		t.Errorf("DeleteExpiredSessions = %d, %v", n, err)
	}
	if err := s.DeleteSession(ctx, "live"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSession(ctx, "live"); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted session returned: %v", err)
	}
}
