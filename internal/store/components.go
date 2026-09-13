package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Component is a part of the service shown on the public status page.
type Component struct {
	ID          int64
	Name        string
	Description string
	Position    int
	CreatedAt   time.Time
}

// CreateComponent adds a component at the end of the list.
func (s *Store) CreateComponent(ctx context.Context, c Component) (Component, error) {
	now := s.now().UTC().Truncate(time.Second)
	c.Name = strings.TrimSpace(c.Name)
	c.Description = strings.TrimSpace(c.Description)
	c.CreatedAt = now
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) + 1 FROM components`).Scan(&c.Position)
	if err != nil {
		return Component{}, err
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO components (name, description, position, created_at) VALUES (?, ?, ?, ?)`,
		c.Name, c.Description, c.Position, now.Unix())
	if err != nil {
		return Component{}, err
	}
	c.ID, err = res.LastInsertId()
	return c, err
}

// UpdateComponent saves the name, description and position.
func (s *Store) UpdateComponent(ctx context.Context, c Component) error {
	res, err := s.db.ExecContext(ctx, `UPDATE components SET name = ?, description = ?, position = ? WHERE id = ?`,
		strings.TrimSpace(c.Name), strings.TrimSpace(c.Description), c.Position, c.ID)
	if err != nil {
		return err
	}
	return expectOneRow(res)
}

// DeleteComponent removes a component. Its monitors stay but are no longer
// shown on the status page.
func (s *Store) DeleteComponent(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM components WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return expectOneRow(res)
}

// GetComponent returns one component.
func (s *Store) GetComponent(ctx context.Context, id int64) (Component, error) {
	var (
		c       Component
		created int64
	)
	err := s.db.QueryRowContext(ctx, `SELECT id, name, description, position, created_at FROM components WHERE id = ?`, id).
		Scan(&c.ID, &c.Name, &c.Description, &c.Position, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Component{}, ErrNotFound
	}
	c.CreatedAt = time.Unix(created, 0).UTC()
	return c, err
}

// ListComponents returns the components in display order.
func (s *Store) ListComponents(ctx context.Context) ([]Component, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, description, position, created_at
		FROM components ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Component
	for rows.Next() {
		var (
			c       Component
			created int64
		)
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Position, &created); err != nil {
			return nil, err
		}
		c.CreatedAt = time.Unix(created, 0).UTC()
		out = append(out, c)
	}
	return out, rows.Err()
}

// CountComponentMonitors returns how many monitors each component has.
func (s *Store) CountComponentMonitors(ctx context.Context) (map[int64]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT component_id, COUNT(*) FROM monitors
		WHERE component_id IS NOT NULL GROUP BY component_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int{}
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}
