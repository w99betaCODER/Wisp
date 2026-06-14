// Package server wires the HTTP API together.
package server

import (
	"crypto/rand"
	"io/fs"
	"net/http"

	"github.com/w99betaCODER/Wisp/internal/cluster"
	"github.com/w99betaCODER/Wisp/internal/config"
	"github.com/w99betaCODER/Wisp/internal/store"
	"github.com/w99betaCODER/Wisp/internal/xray"
	"github.com/w99betaCODER/Wisp/web"
)

// Server holds the dependencies shared by all HTTP handlers.
type Server struct {
	cfg           config.Config
	store         store.Store
	xray          xray.Client
	cluster       *cluster.Cluster
	sessionSecret []byte // signs session cookies (HMAC-SHA256)
}

// New constructs a Server with its dependencies injected.
func New(cfg config.Config, st store.Store, xc xray.Client, cl *cluster.Cluster) *Server {
	return &Server{
		cfg:           cfg,
		store:         st,
		xray:          xc,
		cluster:       cl,
		sessionSecret: sessionSecret(cfg),
	}
}

// sessionSecret returns the configured cookie-signing key, or a random one when
// WISP_SESSION_SECRET is unset (sessions then reset on each restart).
func sessionSecret(cfg config.Config) []byte {
	if cfg.SessionSecret != "" {
		return []byte(cfg.SessionSecret)
	}
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return b
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
	mux.HandleFunc("POST /api/users/{id}/reset", s.handleResetUser)
	mux.HandleFunc("POST /api/users/{id}/topup", s.handleTopUp)
	mux.HandleFunc("POST /api/users/{id}/autorenew", s.handleSetAutoRenew)

	mux.HandleFunc("GET /api/nodes", s.handleListNodes)
	mux.HandleFunc("POST /api/nodes", s.handleCreateNode)
	mux.HandleFunc("GET /api/nodes/{id}", s.handleGetNode)
	mux.HandleFunc("DELETE /api/nodes/{id}", s.handleDeleteNode)

	mux.HandleFunc("GET /api/plans", s.handleListPlans)
	mux.HandleFunc("POST /api/plans", s.handleCreatePlan)
	mux.HandleFunc("DELETE /api/plans/{id}", s.handleDeletePlan)

	// Admin management (super-admin only; enforced inside the handlers).
	mux.HandleFunc("GET /api/admins", s.handleListAdmins)
	mux.HandleFunc("POST /api/admins", s.handleCreateAdmin)
	mux.HandleFunc("DELETE /api/admins/{id}", s.handleDeleteAdmin)

	mux.HandleFunc("GET /api/orders", s.handleListOrders)
	mux.HandleFunc("POST /api/orders", s.handleCreateOrder)
	mux.HandleFunc("POST /api/orders/{id}/pay", s.handlePayOrder)

	// Payment-gateway callback (HMAC-authenticated, not token-gated).
	mux.HandleFunc("POST /api/webhook/{provider}", s.handleWebhook)

	// Public endpoints: branding for the login page, and login itself.
	mux.HandleFunc("GET /api/branding", s.handleBranding)
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)

	// Identity of the signed-in admin (drives the role-aware dashboard).
	mux.HandleFunc("GET /api/me", s.handleMe)

	// Subscription link consumed directly by VPN clients.
	mux.HandleFunc("GET /sub/{id}", s.handleSubscription)

	// Embedded dashboard served at "/" (the API patterns above are more
	// specific, so they always win over this catch-all).
	static, err := fs.Sub(web.FS(), "static")
	if err != nil {
		panic(err) // the embedded FS is built at compile time; this cannot fail
	}
	mux.Handle("GET /", noCache(http.FileServerFS(static)))

	// Requests pass through logging, then the API-token gate.
	return logging(s.auth(mux))
}
