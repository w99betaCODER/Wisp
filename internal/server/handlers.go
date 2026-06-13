package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/w99betaCODER/Wisp/internal/model"
	"github.com/w99betaCODER/Wisp/internal/store"
	"github.com/w99betaCODER/Wisp/internal/subscription"
	"github.com/w99betaCODER/Wisp/internal/util"
)

// handleHealth is a liveness probe used by Docker, load balancers and uptime checks.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleListUsers returns every user.
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

// createUserRequest is the JSON body accepted by POST /api/users.
type createUserRequest struct {
	Email     string     `json:"email"`
	DataLimit int64      `json:"data_limit"` // bytes; 0 or omitted = unlimited
	ExpiresAt *time.Time `json:"expires_at"` // RFC3339; omitted = never expires
}

// handleCreateUser creates a new VPN user, generating its id and UUID.
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	user := model.User{
		ID:        util.NewID(),
		Email:     req.Email,
		UUID:      util.NewUUID(),
		Enabled:   true,
		DataLimit: req.DataLimit,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: req.ExpiresAt,
	}
	if err := s.store.CreateUser(user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// Push the client into Xray so it can connect immediately. If Xray rejects
	// it, roll back the DB insert so the two never drift out of sync.
	if err := s.xray.AddUser(r.Context(), s.cfg.InboundTag, user.Email, user.UUID, s.cfg.Node.Flow); err != nil {
		_ = s.store.DeleteUser(user.ID)
		log.Printf("create user: xray add failed, rolled back: %v", err)
		writeError(w, http.StatusBadGateway, "failed to add user to xray")
		return
	}

	// Fan the new user out to every enabled node (best-effort).
	s.cluster.AddUser(r.Context(), user.Email, user.UUID, s.cfg.Node.Flow)

	writeJSON(w, http.StatusCreated, user)
}

// handleGetUser returns a single user by id.
func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user, err := s.store.GetUser(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// handleDeleteUser removes a user by id.
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Fetch first so we know the email Xray needs to drop the client.
	user, err := s.store.GetUser(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user")
		return
	}

	if err := s.store.DeleteUser(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}

	// The user is already gone from the DB; a failure here only means a stale
	// entry lingers in Xray, so we log it rather than failing the request.
	if err := s.xray.RemoveUser(r.Context(), s.cfg.InboundTag, user.Email); err != nil {
		log.Printf("delete user: xray remove failed: %v", err)
	}

	// Remove from every enabled node too (best-effort).
	s.cluster.RemoveUser(r.Context(), user.Email)

	w.WriteHeader(http.StatusNoContent)
}

// handleResetUser zeroes a user's traffic counter and re-enables them, then
// re-adds them to the local Xray and every node. Use this after a renewal or
// a quota top-up to bring a disabled user back online.
func (s *Server) handleResetUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user, err := s.store.GetUser(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user")
		return
	}

	wasDisabled := !user.Enabled
	user.Used = 0
	user.Enabled = true
	if err := s.store.UpdateUser(user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	// If the user had been disabled, they were removed from the proxies — add
	// them back so the reset actually restores access.
	if wasDisabled {
		if err := s.xray.AddUser(r.Context(), s.cfg.InboundTag, user.Email, user.UUID, s.cfg.Node.Flow); err != nil {
			log.Printf("reset user: local xray add failed: %v", err)
		}
		s.cluster.AddUser(r.Context(), user.Email, user.UUID, s.cfg.Node.Flow)
	}
	writeJSON(w, http.StatusOK, user)
}

// handleSubscription returns the base64-encoded share links for a user, the
// content a VPN client fetches from its subscription URL.
//
// NOTE: the user id doubles as the subscription token for now. A dedicated,
// unguessable token is a Phase 2 hardening task.
func (s *Server) handleSubscription(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user, err := s.store.GetUser(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "subscription not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load subscription")
		return
	}

	link := subscription.VLESSLink(user, s.cfg.Node)
	content := subscription.Encode([]string{link})

	// Plain text + headers that clients read for the profile name.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Profile-Title", "Wisp")
	if _, err := w.Write([]byte(content)); err != nil {
		log.Printf("subscription write: %v", err)
	}
}
