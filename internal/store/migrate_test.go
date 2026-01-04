package store

import (
	"context"
	"path/filepath"
	"testing"
)

// newTestStore returns a migrated store backed by a temporary file.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMigrateIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("second run: %v", err)
	}
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != len(migrations) {
		t.Errorf("applied %d migrations, have %d", n, len(migrations))
	}
}

func TestMigrationsAreOrdered(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for i, m := range migrations {
		if m.version != i+1 {
			t.Errorf("migration %s has version %d, want %d", m.name, m.version, i+1)
		}
	}
}
