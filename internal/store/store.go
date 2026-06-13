// Package store defines how the panel persists its data and provides an
// in-memory implementation.
//
// The rest of the app depends on the Store interface, never on a concrete
// type. That means a SQLite- or Postgres-backed store can be dropped in
// later without touching any handler code.
package store

import (
	"errors"
	"sort"
	"sync"

	"github.com/w99betaCODER/Wisp/internal/model"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// Store is the persistence contract the panel depends on.
type Store interface {
	ListUsers() ([]model.User, error)
	GetUser(id string) (model.User, error)
	CreateUser(u model.User) error
	UpdateUser(u model.User) error
	DeleteUser(id string) error

	ListNodes() ([]model.Node, error)
	GetNode(id string) (model.Node, error)
	CreateNode(n model.Node) error
	DeleteNode(id string) error
}

// MemoryStore is a thread-safe, in-process Store. Data is lost on restart;
// it exists so the panel runs with zero setup during early development.
type MemoryStore struct {
	mu    sync.RWMutex
	users map[string]model.User
	nodes map[string]model.Node
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users: make(map[string]model.User),
		nodes: make(map[string]model.Node),
	}
}

// ListUsers returns all users ordered by creation time (oldest first).
func (s *MemoryStore) ListUsers() ([]model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]model.User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	// Stable ordering keeps the API predictable for clients and tests.
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// GetUser returns the user with the given id, or ErrNotFound.
func (s *MemoryStore) GetUser(id string) (model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.users[id]
	if !ok {
		return model.User{}, ErrNotFound
	}
	return u, nil
}

// CreateUser stores a new user (or overwrites one with the same id).
func (s *MemoryStore) CreateUser(u model.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.users[u.ID] = u
	return nil
}

// UpdateUser replaces the stored user with the same id, or ErrNotFound.
func (s *MemoryStore) UpdateUser(u model.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[u.ID]; !ok {
		return ErrNotFound
	}
	s.users[u.ID] = u
	return nil
}

// DeleteUser removes a user by id, returning ErrNotFound if absent.
func (s *MemoryStore) DeleteUser(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[id]; !ok {
		return ErrNotFound
	}
	delete(s.users, id)
	return nil
}

// ListNodes returns all nodes ordered by creation time (oldest first).
func (s *MemoryStore) ListNodes() ([]model.Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]model.Node, 0, len(s.nodes))
	for _, n := range s.nodes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// GetNode returns the node with the given id, or ErrNotFound.
func (s *MemoryStore) GetNode(id string) (model.Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n, ok := s.nodes[id]
	if !ok {
		return model.Node{}, ErrNotFound
	}
	return n, nil
}

// CreateNode stores a new node (or overwrites one with the same id).
func (s *MemoryStore) CreateNode(n model.Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nodes[n.ID] = n
	return nil
}

// DeleteNode removes a node by id, returning ErrNotFound if absent.
func (s *MemoryStore) DeleteNode(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.nodes[id]; !ok {
		return ErrNotFound
	}
	delete(s.nodes, id)
	return nil
}
