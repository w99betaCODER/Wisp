package store

import (
	"database/sql"
	"fmt"
	"strings"
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
    data_limit INTEGER NOT NULL DEFAULT 0,
    used       INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP
);
CREATE TABLE IF NOT EXISTS nodes (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    address    TEXT NOT NULL,
    enabled    INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL
);`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// Idempotent column additions for databases created before these columns
	// existed. On a fresh DB the columns are already present, so SQLite reports
	// "duplicate column name", which we deliberately ignore.
	for _, ddl := range []string{
		`ALTER TABLE users ADD COLUMN data_limit INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN used INTEGER NOT NULL DEFAULT 0`,
	} {
		if _, err := s.db.Exec(ddl); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// ListUsers returns all users ordered by creation time (oldest first).
func (s *SQLiteStore) ListUsers() ([]model.User, error) {
	rows, err := s.db.Query(`
		SELECT id, email, uuid, enabled, data_limit, used, created_at, expires_at
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
		SELECT id, email, uuid, enabled, data_limit, used, created_at, expires_at
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
		INSERT INTO users (id, email, uuid, enabled, data_limit, used, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.UUID, u.Enabled, u.DataLimit, u.Used, u.CreatedAt, nullTime(u.ExpiresAt))
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// UpdateUser persists changes to an existing user (or ErrNotFound).
func (s *SQLiteStore) UpdateUser(u model.User) error {
	res, err := s.db.Exec(`
		UPDATE users
		SET email = ?, uuid = ?, enabled = ?, data_limit = ?, used = ?, expires_at = ?
		WHERE id = ?`,
		u.Email, u.UUID, u.Enabled, u.DataLimit, u.Used, nullTime(u.ExpiresAt), u.ID)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
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

// ListNodes returns all nodes ordered by creation time (oldest first).
func (s *SQLiteStore) ListNodes() ([]model.Node, error) {
	rows, err := s.db.Query(`
		SELECT id, name, address, enabled, created_at
		FROM nodes ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	defer rows.Close()

	var out []model.Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// GetNode returns the node with the given id, or ErrNotFound.
func (s *SQLiteStore) GetNode(id string) (model.Node, error) {
	row := s.db.QueryRow(`
		SELECT id, name, address, enabled, created_at
		FROM nodes WHERE id = ?`, id)

	n, err := scanNode(row)
	if err == sql.ErrNoRows {
		return model.Node{}, ErrNotFound
	}
	if err != nil {
		return model.Node{}, fmt.Errorf("get node: %w", err)
	}
	return n, nil
}

// CreateNode inserts a new node.
func (s *SQLiteStore) CreateNode(n model.Node) error {
	_, err := s.db.Exec(`
		INSERT INTO nodes (id, name, address, enabled, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		n.ID, n.Name, n.Address, n.Enabled, n.CreatedAt)
	if err != nil {
		return fmt.Errorf("create node: %w", err)
	}
	return nil
}

// DeleteNode removes a node by id, returning ErrNotFound if absent.
func (s *SQLiteStore) DeleteNode(id string) error {
	res, err := s.db.Exec(`DELETE FROM nodes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete node: %w", err)
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

// scanNode reads one node row.
func scanNode(sc scanner) (model.Node, error) {
	var n model.Node
	if err := sc.Scan(&n.ID, &n.Name, &n.Address, &n.Enabled, &n.CreatedAt); err != nil {
		return model.Node{}, err
	}
	return n, nil
}

// scanUser reads one user row, translating the nullable expires_at column.
func scanUser(sc scanner) (model.User, error) {
	var (
		u       model.User
		expires sql.NullTime
	)
	if err := sc.Scan(&u.ID, &u.Email, &u.UUID, &u.Enabled, &u.DataLimit, &u.Used, &u.CreatedAt, &expires); err != nil {
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
