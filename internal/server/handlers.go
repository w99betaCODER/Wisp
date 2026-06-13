package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/wisp-panel/wisp/internal/model"
	"github.com/wisp-panel/wisp/internal/store"
	"github.com/wisp-panel/wisp/internal/util"
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
	Email string `json:"email"`
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
		CreatedAt: time.Now().UTC(),
	}
	if err := s.store.CreateUser(user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}
	// TODO(phase 1): push this client into Xray over gRPC so it works immediately.
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
	err := s.store.DeleteUser(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	// TODO(phase 1): remove this client from Xray over gRPC.
	w.WriteHeader(http.StatusNoContent)
}
