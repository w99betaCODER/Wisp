// Package server wires the HTTP API together.
package server

import (
	"net/http"

	"github.com/w99betaCODER/Wisp/internal/config"
	"github.com/w99betaCODER/Wisp/internal/store"
	"github.com/w99betaCODER/Wisp/internal/xray"
)

// Server holds the dependencies shared by all HTTP handlers.
type Server struct {
	cfg   config.Config
	store store.Store
	xray  xray.Client
}

// New constructs a Server with its dependencies injected.
func New(cfg config.Config, st store.Store, xc xray.Client) *Server {
	return &Server{cfg: cfg, store: st, xray: xc}
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

	// Subscription link consumed directly by VPN clients.
	mux.HandleFunc("GET /sub/{id}", s.handleSubscription)

	// Every request passes through the logging middleware first.
	return logging(mux)
}
