package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ErrDuplicate is returned when a unique value is already taken.
var ErrDuplicate = errors.New("already exists")

// User is an administrator account.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

// CreateUser adds an administrator.
func (s *Store) CreateUser(ctx context.Context, username, passwordHash string) (User, error) {
	now := s.now().UTC().Truncate(time.Second)
	res, err := s.db.ExecContext(ctx, `INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, passwordHash, now.Unix())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return User{}, ErrDuplicate
		}
		return User{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, err
	}
	return User{ID: id, Username: username, PasswordHash: passwordHash, CreatedAt: now}, nil
}

// GetUserByUsername looks a user up by name, ignoring case.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (User, error) {
	return s.getUser(ctx, `SELECT id, username, password_hash, created_at FROM users WHERE username = ?`, username)
}

// GetUser looks a user up by ID.
func (s *Store) GetUser(ctx context.Context, id int64) (User, error) {
	return s.getUser(ctx, `SELECT id, username, password_hash, created_at FROM users WHERE id = ?`, id)
}

func (s *Store) getUser(ctx context.Context, query string, arg any) (User, error) {
	var (
		u       User
		created int64
	)
	err := s.db.QueryRowContext(ctx, query, arg).Scan(&u.ID, &u.Username, &u.PasswordHash, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	u.CreatedAt = time.Unix(created, 0).UTC()
	return u, nil
}

// CountUsers returns the number of accounts.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// UpdatePassword stores a new hash and logs the user out everywhere except
// in the session given by keepTokenHash.
func (s *Store) UpdatePassword(ctx context.Context, userID int64, passwordHash, keepTokenHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
	if err != nil {
		return err
	}
	if err := expectOneRow(res); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND token_hash != ?`, userID, keepTokenHash); err != nil {
		return err
	}
	return tx.Commit()
}
