package store

import (
	"context"
	"errors"
	"testing"
)

func TestUsers(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	u, err := s.CreateUser(ctx, "admin", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(ctx, "ADMIN", "other"); !errors.Is(err, ErrDuplicate) {
		t.Errorf("expected ErrDuplicate, got %v", err)
	}
	got, err := s.GetUserByUsername(ctx, "Admin")
	if err != nil || got.ID != u.ID || got.PasswordHash != "hash" {
		t.Errorf("lookup by name: %+v, %v", got, err)
	}
	if _, err := s.GetUser(ctx, 99); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if n, _ := s.CountUsers(ctx); n != 1 {
		t.Errorf("CountUsers = %d", n)
	}
}
