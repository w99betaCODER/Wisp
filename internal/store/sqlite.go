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
    balance    INTEGER NOT NULL DEFAULT 0,
    auto_renew TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP
);
CREATE TABLE IF NOT EXISTS nodes (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    address     TEXT NOT NULL,
    protocol    TEXT NOT NULL DEFAULT 'vless',
    public_host TEXT NOT NULL DEFAULT '',
    public_port INTEGER NOT NULL DEFAULT 443,
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TIMESTAMP NOT NULL
);
CREATE TABLE IF NOT EXISTS plans (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    price_cents   INTEGER NOT NULL DEFAULT 0,
    currency      TEXT NOT NULL DEFAULT 'USD',
    duration_days INTEGER NOT NULL DEFAULT 30,
    data_limit    INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMP NOT NULL
);
CREATE TABLE IF NOT EXISTS orders (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL,
    plan_id      TEXT NOT NULL,
    amount_cents INTEGER NOT NULL DEFAULT 0,
    currency     TEXT NOT NULL DEFAULT 'USD',
    status       TEXT NOT NULL DEFAULT 'pending',
    provider     TEXT NOT NULL DEFAULT 'manual',
    created_at   TIMESTAMP NOT NULL,
    paid_at      TIMESTAMP
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
		`ALTER TABLE users ADD COLUMN balance INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN auto_renew TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE nodes ADD COLUMN protocol TEXT NOT NULL DEFAULT 'vless'`,
		`ALTER TABLE nodes ADD COLUMN public_host TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE nodes ADD COLUMN public_port INTEGER NOT NULL DEFAULT 443`,
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
		SELECT id, email, uuid, enabled, data_limit, used, balance, auto_renew, created_at, expires_at
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
		SELECT id, email, uuid, enabled, data_limit, used, balance, auto_renew, created_at, expires_at
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
		INSERT INTO users (id, email, uuid, enabled, data_limit, used, balance, auto_renew, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.UUID, u.Enabled, u.DataLimit, u.Used, u.Balance, u.AutoRenew, u.CreatedAt, nullTime(u.ExpiresAt))
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// UpdateUser persists changes to an existing user (or ErrNotFound).
func (s *SQLiteStore) UpdateUser(u model.User) error {
	res, err := s.db.Exec(`
		UPDATE users
		SET email = ?, uuid = ?, enabled = ?, data_limit = ?, used = ?, balance = ?, auto_renew = ?, expires_at = ?
		WHERE id = ?`,
		u.Email, u.UUID, u.Enabled, u.DataLimit, u.Used, u.Balance, u.AutoRenew, nullTime(u.ExpiresAt), u.ID)
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
		SELECT id, name, address, protocol, public_host, public_port, enabled, created_at
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
		SELECT id, name, address, protocol, public_host, public_port, enabled, created_at
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
		INSERT INTO nodes (id, name, address, protocol, public_host, public_port, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Name, n.Address, n.Protocol, n.PublicHost, n.PublicPort, n.Enabled, n.CreatedAt)
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

// ListPlans returns all plans ordered by creation time (oldest first).
func (s *SQLiteStore) ListPlans() ([]model.Plan, error) {
	rows, err := s.db.Query(`
		SELECT id, name, price_cents, currency, duration_days, data_limit, created_at
		FROM plans ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}
	defer rows.Close()

	var out []model.Plan
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPlan returns the plan with the given id, or ErrNotFound.
func (s *SQLiteStore) GetPlan(id string) (model.Plan, error) {
	row := s.db.QueryRow(`
		SELECT id, name, price_cents, currency, duration_days, data_limit, created_at
		FROM plans WHERE id = ?`, id)

	p, err := scanPlan(row)
	if err == sql.ErrNoRows {
		return model.Plan{}, ErrNotFound
	}
	if err != nil {
		return model.Plan{}, fmt.Errorf("get plan: %w", err)
	}
	return p, nil
}

// CreatePlan inserts a new plan.
func (s *SQLiteStore) CreatePlan(p model.Plan) error {
	_, err := s.db.Exec(`
		INSERT INTO plans (id, name, price_cents, currency, duration_days, data_limit, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.PriceCents, p.Currency, p.DurationDays, p.DataLimit, p.CreatedAt)
	if err != nil {
		return fmt.Errorf("create plan: %w", err)
	}
	return nil
}

// DeletePlan removes a plan by id, returning ErrNotFound if absent.
func (s *SQLiteStore) DeletePlan(id string) error {
	res, err := s.db.Exec(`DELETE FROM plans WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete plan: %w", err)
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

// ListOrders returns all orders ordered by creation time (oldest first).
func (s *SQLiteStore) ListOrders() ([]model.Order, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, plan_id, amount_cents, currency, status, provider, created_at, paid_at
		FROM orders ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	var out []model.Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// GetOrder returns the order with the given id, or ErrNotFound.
func (s *SQLiteStore) GetOrder(id string) (model.Order, error) {
	row := s.db.QueryRow(`
		SELECT id, user_id, plan_id, amount_cents, currency, status, provider, created_at, paid_at
		FROM orders WHERE id = ?`, id)

	o, err := scanOrder(row)
	if err == sql.ErrNoRows {
		return model.Order{}, ErrNotFound
	}
	if err != nil {
		return model.Order{}, fmt.Errorf("get order: %w", err)
	}
	return o, nil
}

// CreateOrder inserts a new order.
func (s *SQLiteStore) CreateOrder(o model.Order) error {
	_, err := s.db.Exec(`
		INSERT INTO orders (id, user_id, plan_id, amount_cents, currency, status, provider, created_at, paid_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.ID, o.UserID, o.PlanID, o.AmountCents, o.Currency, o.Status, o.Provider, o.CreatedAt, nullTime(o.PaidAt))
	if err != nil {
		return fmt.Errorf("create order: %w", err)
	}
	return nil
}

// UpdateOrder persists changes to an existing order (or ErrNotFound).
func (s *SQLiteStore) UpdateOrder(o model.Order) error {
	res, err := s.db.Exec(`
		UPDATE orders SET status = ?, provider = ?, paid_at = ? WHERE id = ?`,
		o.Status, o.Provider, nullTime(o.PaidAt), o.ID)
	if err != nil {
		return fmt.Errorf("update order: %w", err)
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

// scanPlan reads one plan row.
func scanPlan(sc scanner) (model.Plan, error) {
	var p model.Plan
	if err := sc.Scan(&p.ID, &p.Name, &p.PriceCents, &p.Currency, &p.DurationDays, &p.DataLimit, &p.CreatedAt); err != nil {
		return model.Plan{}, err
	}
	return p, nil
}

// scanOrder reads one order row, translating the nullable paid_at column.
func scanOrder(sc scanner) (model.Order, error) {
	var (
		o    model.Order
		paid sql.NullTime
	)
	if err := sc.Scan(&o.ID, &o.UserID, &o.PlanID, &o.AmountCents, &o.Currency, &o.Status, &o.Provider, &o.CreatedAt, &paid); err != nil {
		return model.Order{}, err
	}
	if paid.Valid {
		o.PaidAt = &paid.Time
	}
	return o, nil
}

// scanNode reads one node row.
func scanNode(sc scanner) (model.Node, error) {
	var n model.Node
	if err := sc.Scan(&n.ID, &n.Name, &n.Address, &n.Protocol, &n.PublicHost, &n.PublicPort, &n.Enabled, &n.CreatedAt); err != nil {
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
	if err := sc.Scan(&u.ID, &u.Email, &u.UUID, &u.Enabled, &u.DataLimit, &u.Used, &u.Balance, &u.AutoRenew, &u.CreatedAt, &expires); err != nil {
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
