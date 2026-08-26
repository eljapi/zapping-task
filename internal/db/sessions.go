package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateSession(ctx context.Context, id string, userID int64, expiresAt time.Time) error {
	query := `INSERT INTO sessions (id, user_id, expires_at)
	          VALUES ($1, $2, $3)`

	if _, err := s.pool.Exec(ctx, query, id, userID, expiresAt); err != nil {
		return fmt.Errorf("creating session: %w", err)
	}

	return nil
}

func (s *Store) SessionUser(ctx context.Context, sessionID string) (*User, error) {
	query := `SELECT u.id, u.name, u.email, u.password_hash
	          FROM sessions s
	          JOIN users u ON u.id = s.user_id
	          WHERE s.id = $1 AND s.expires_at > now()`

	user := &User{}
	err := s.pool.QueryRow(ctx, query, sessionID).
		Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("looking up session: %w", err)
	}

	return user, nil
}

func (s *Store) DeleteSession(ctx context.Context, sessionID string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}

	return nil
}
