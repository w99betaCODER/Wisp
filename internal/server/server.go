// Package server wires the HTTP API together.
package server

import (
	"net/http"

	"github.com/wisp-panel/wisp/internal/config"
	"github.com/wisp-panel/wisp/internal/store"
)

// Server holds the dependencies shared by all HTTP handlers.
type Server struct {
	cfg   config.Config
	store store.Store
}

// New constructs a Server with its dependencies injected.
func New(cfg config.Config, st store.Store) *Server {
	return &Server{cfg: cfg, store: st}
}

// Routes builds the HTTP handler with every route registered.
//
// Go 1.22+ lets us put the HTTP method directly in the pattern
// ("GET /api/users"), and read path parameters via r.PathValue("id"),
// so we need no third-party router.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealth)

	mux.HandleFunc("GET /api/users", s.handleListUsers)
	mux.HandleFunc("POST /api/users", s.handleCreateUser)
	mux.HandleFunc("GET /api/users/{id}", s.handleGetUser)
	mux.HandleFunc("DELETE /api/users/{id}", s.handleDeleteUser)

	// Every request passes through the logging middleware first.
	return logging(mux)
}
