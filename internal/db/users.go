package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type User struct {
	ID           int64
	Name         string
	Email        string
	PasswordHash string
}

/*Postgres SQLSTATE for a unique constraint violation*/
const uniqueViolation = "23505"

func (s *Store) CreateUser(ctx context.Context, name, email, passwordHash string) (*User, error) {
	user := &User{Name: name, Email: normalizeEmail(email), PasswordHash: passwordHash}

	query := `INSERT INTO users (name, email, password_hash)
	          VALUES ($1, $2, $3)
	          RETURNING id`

	/*
		$1/$2/$3 are placeholders, not string interpolation: the values travel to
		Postgres apart from the statement and are never parsed as SQL, which is
		what actually stops injection
	*/
	err := s.pool.QueryRow(ctx, query, user.Name, user.Email, user.PasswordHash).Scan(&user.ID)
	if err != nil {
		/*
			errors.As unwraps the chain until it finds a *pgconn.PgError. A duplicate
			email is not a failure of the system, it is an answer, so we turn it into
			a domain error the handler can act on
		*/
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("creating user: %w", err)
	}

	return user, nil
}

func (s *Store) UserByEmail(ctx context.Context, email string) (*User, error) {
	query := `SELECT id, name, email, password_hash
	          FROM users
	          WHERE email = $1`

	user := &User{}
	err := s.pool.QueryRow(ctx, query, normalizeEmail(email)).
		Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	return user, nil
}

/*
The unique index is on the stored value, so the same address typed with
different casing has to normalise to the same string or it would register twice
*/
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
