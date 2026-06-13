package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/w99betaCODER/Wisp/internal/model"

	// Pure-Go SQLite driver (no cgo) — keeps Wisp a single static binary.
	_ "modernc.org/sqlite"
)

// SQLiteStore is a Store backed by a SQLite database file. It implements the
// exact same interface as MemoryStore, so swapping it in changes nothing in
// the handlers — that is the whole point of programming to the interface.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) the database at path and applies the
// schema. Pass ":memory:" for an ephemeral database.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite allows only one writer; cap the pool so writes serialize cleanly.
	db.SetMaxOpenConns(1)

	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the underlying database handle.
func (s *SQLiteStore) Close() error { return s.db.Close() }

// migrate creates the schema if it does not already exist.
func (s *SQLiteStore) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS users (
    id         TEXT PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    uuid       TEXT NOT NULL,
    enabled    INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP
);`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// ListUsers returns all users ordered by creation time (oldest first).
func (s *SQLiteStore) ListUsers() ([]model.User, error) {
	rows, err := s.db.Query(`
		SELECT id, email, uuid, enabled, created_at, expires_at
		FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var out []model.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// GetUser returns the user with the given id, or ErrNotFound.
func (s *SQLiteStore) GetUser(id string) (model.User, error) {
	row := s.db.QueryRow(`
		SELECT id, email, uuid, enabled, created_at, expires_at
		FROM users WHERE id = ?`, id)

	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return model.User{}, ErrNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

// CreateUser inserts a new user.
func (s *SQLiteStore) CreateUser(u model.User) error {
	_, err := s.db.Exec(`
		INSERT INTO users (id, email, uuid, enabled, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.UUID, u.Enabled, u.CreatedAt, nullTime(u.ExpiresAt))
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// DeleteUser removes a user by id, returning ErrNotFound if absent.
func (s *SQLiteStore) DeleteUser(id string) error {
	res, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows, so scanUser works
// for single-row and multi-row queries alike.
type scanner interface {
	Scan(dest ...any) error
}

// scanUser reads one user row, translating the nullable expires_at column.
func scanUser(sc scanner) (model.User, error) {
	var (
		u       model.User
		expires sql.NullTime
	)
	if err := sc.Scan(&u.ID, &u.Email, &u.UUID, &u.Enabled, &u.CreatedAt, &expires); err != nil {
		return model.User{}, err
	}
	if expires.Valid {
		u.ExpiresAt = &expires.Time
	}
	return u, nil
}

// nullTime converts a *time.Time into a sql.NullTime for parameter binding.
func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}
