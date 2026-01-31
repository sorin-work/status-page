package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Session is a logged in admin.
type Session struct {
	TokenHash string
	UserID    int64
	Username  string
	CSRFToken string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// CreateSession stores a new session.
func (s *Store) CreateSession(ctx context.Context, sess Session) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, csrf_token, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?)`,
		sess.TokenHash, sess.UserID, sess.CSRFToken, sess.CreatedAt.Unix(), sess.ExpiresAt.Unix())
	return err
}

// GetSession returns an unexpired session by token hash.
func (s *Store) GetSession(ctx context.Context, tokenHash string) (Session, error) {
	var (
		sess             Session
		created, expires int64
	)
	err := s.db.QueryRowContext(ctx, `SELECT s.token_hash, s.user_id, u.username, s.csrf_token, s.created_at, s.expires_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, s.now().Unix()).
		Scan(&sess.TokenHash, &sess.UserID, &sess.Username, &sess.CSRFToken, &created, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	sess.CreatedAt = time.Unix(created, 0).UTC()
	sess.ExpiresAt = time.Unix(expires, 0).UTC()
	return sess, nil
}

// DeleteSession removes a session.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// DeleteExpiredSessions removes sessions that have expired.
func (s *Store) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, s.now().Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
