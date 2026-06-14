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

	ListPlans() ([]model.Plan, error)
	GetPlan(id string) (model.Plan, error)
	CreatePlan(p model.Plan) error
	DeletePlan(id string) error

	ListOrders() ([]model.Order, error)
	GetOrder(id string) (model.Order, error)
	CreateOrder(o model.Order) error
	UpdateOrder(o model.Order) error

	ListAdmins() ([]model.Admin, error)
	GetAdmin(id string) (model.Admin, error)
	GetAdminByUsername(username string) (model.Admin, error)
	CreateAdmin(a model.Admin) error
	DeleteAdmin(id string) error

	// ListUsersByOwner returns only the users owned by ownerID.
	ListUsersByOwner(ownerID string) ([]model.User, error)
}

// MemoryStore is a thread-safe, in-process Store. Data is lost on restart;
// it exists so the panel runs with zero setup during early development.
type MemoryStore struct {
	mu     sync.RWMutex
	users  map[string]model.User
	nodes  map[string]model.Node
	plans  map[string]model.Plan
	orders map[string]model.Order
	admins map[string]model.Admin
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:  make(map[string]model.User),
		nodes:  make(map[string]model.Node),
		plans:  make(map[string]model.Plan),
		orders: make(map[string]model.Order),
		admins: make(map[string]model.Admin),
	}
}

// ListUsersByOwner returns the owner's users, oldest first.
func (s *MemoryStore) ListUsersByOwner(ownerID string) ([]model.User, error) {
	all, _ := s.ListUsers()
	out := make([]model.User, 0)
	for _, u := range all {
		if u.OwnerID == ownerID {
			out = append(out, u)
		}
	}
	return out, nil
}

// ListAdmins returns all admins ordered by creation time (oldest first).
func (s *MemoryStore) ListAdmins() ([]model.Admin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]model.Admin, 0, len(s.admins))
	for _, a := range s.admins {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

// GetAdmin returns the admin with the given id, or ErrNotFound.
func (s *MemoryStore) GetAdmin(id string) (model.Admin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	a, ok := s.admins[id]
	if !ok {
		return model.Admin{}, ErrNotFound
	}
	return a, nil
}

// GetAdminByUsername returns the admin with the given username, or ErrNotFound.
func (s *MemoryStore) GetAdminByUsername(username string) (model.Admin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, a := range s.admins {
		if a.Username == username {
			return a, nil
		}
	}
	return model.Admin{}, ErrNotFound
}

// CreateAdmin stores a new admin.
func (s *MemoryStore) CreateAdmin(a model.Admin) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.admins[a.ID] = a
	return nil
}

// DeleteAdmin removes an admin by id, returning ErrNotFound if absent.
func (s *MemoryStore) DeleteAdmin(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.admins[id]; !ok {
		return ErrNotFound
	}
	delete(s.admins, id)
	return nil
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

// ListPlans returns all plans ordered by creation time (oldest first).
func (s *MemoryStore) ListPlans() ([]model.Plan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]model.Plan, 0, len(s.plans))
	for _, p := range s.plans {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// GetPlan returns the plan with the given id, or ErrNotFound.
func (s *MemoryStore) GetPlan(id string) (model.Plan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.plans[id]
	if !ok {
		return model.Plan{}, ErrNotFound
	}
	return p, nil
}

// CreatePlan stores a new plan.
func (s *MemoryStore) CreatePlan(p model.Plan) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.plans[p.ID] = p
	return nil
}

// DeletePlan removes a plan by id, returning ErrNotFound if absent.
func (s *MemoryStore) DeletePlan(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.plans[id]; !ok {
		return ErrNotFound
	}
	delete(s.plans, id)
	return nil
}

// ListOrders returns all orders ordered by creation time (oldest first).
func (s *MemoryStore) ListOrders() ([]model.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]model.Order, 0, len(s.orders))
	for _, o := range s.orders {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// GetOrder returns the order with the given id, or ErrNotFound.
func (s *MemoryStore) GetOrder(id string) (model.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	o, ok := s.orders[id]
	if !ok {
		return model.Order{}, ErrNotFound
	}
	return o, nil
}

// CreateOrder stores a new order.
func (s *MemoryStore) CreateOrder(o model.Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.orders[o.ID] = o
	return nil
}

// UpdateOrder replaces an existing order with the same id, or ErrNotFound.
func (s *MemoryStore) UpdateOrder(o model.Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.orders[o.ID]; !ok {
		return ErrNotFound
	}
	s.orders[o.ID] = o
	return nil
}
