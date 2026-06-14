package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/w99betaCODER/Wisp/internal/config"
	"github.com/w99betaCODER/Wisp/internal/model"
	"github.com/w99betaCODER/Wisp/internal/store"
	"github.com/w99betaCODER/Wisp/internal/util"
	"golang.org/x/crypto/bcrypt"
)

// handleListAdmins returns every admin (super-admin only).
func (s *Server) handleListAdmins(w http.ResponseWriter, r *http.Request) {
	if !s.requireSuper(w, r) {
		return
	}
	admins, err := s.store.ListAdmins()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list admins")
		return
	}
	writeJSON(w, http.StatusOK, admins)
}

type createAdminRequest struct {
	Username string     `json:"username"`
	Password string     `json:"password"`
	Role     model.Role `json:"role"`
}

// handleCreateAdmin provisions a new admin (super-admin only).
func (s *Server) handleCreateAdmin(w http.ResponseWriter, r *http.Request) {
	if !s.requireSuper(w, r) {
		return
	}
	var req createAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	switch req.Role {
	case "", model.RoleReseller:
		req.Role = model.RoleReseller
	case model.RoleSuper:
	default:
		writeError(w, http.StatusBadRequest, "role must be super or reseller")
		return
	}
	if _, err := s.store.GetAdminByUsername(req.Username); err == nil {
		writeError(w, http.StatusConflict, "username already taken")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	admin := model.Admin{
		ID:           util.NewID(),
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         req.Role,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.store.CreateAdmin(admin); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create admin")
		return
	}
	writeJSON(w, http.StatusCreated, admin)
}

// handleDeleteAdmin removes an admin (super-admin only). The caller cannot
// delete themselves, and the last super-admin cannot be removed.
func (s *Server) handleDeleteAdmin(w http.ResponseWriter, r *http.Request) {
	if !s.requireSuper(w, r) {
		return
	}
	id := r.PathValue("id")
	if id == adminFrom(r).ID {
		writeError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}
	target, err := s.store.GetAdmin(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "admin not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load admin")
		return
	}
	if target.Role == model.RoleSuper && s.lastSuper() {
		writeError(w, http.StatusBadRequest, "cannot delete the last super-admin")
		return
	}
	if err := s.store.DeleteAdmin(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete admin")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// lastSuper reports whether exactly one super-admin remains.
func (s *Server) lastSuper() bool {
	admins, err := s.store.ListAdmins()
	if err != nil {
		return true // fail safe: refuse the delete
	}
	n := 0
	for _, a := range admins {
		if a.Role == model.RoleSuper {
			n++
		}
	}
	return n <= 1
}

// Bootstrap seeds (or updates) the super-admin from WISP_ADMIN_USER /
// WISP_ADMIN_PASS. With no password set the panel runs open (dev mode) and no
// admin is created. Calling it on every start lets the env rotate the password.
func Bootstrap(st store.Store, cfg config.Config) error {
	if cfg.AdminPass == "" {
		return nil
	}
	existing, err := st.GetAdminByUsername(cfg.AdminUser)
	if err == nil {
		// Admin already present: refresh the hash if the env password changed.
		if bcrypt.CompareHashAndPassword([]byte(existing.PasswordHash), []byte(cfg.AdminPass)) == nil {
			return nil
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPass), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		existing.PasswordHash = string(hash)
		existing.Role = model.RoleSuper
		return st.CreateAdmin(existing) // upsert (Create overwrites same id)
	}
	if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return st.CreateAdmin(model.Admin{
		ID:           util.NewID(),
		Username:     cfg.AdminUser,
		PasswordHash: string(hash),
		Role:         model.RoleSuper,
		CreatedAt:    time.Now().UTC(),
	})
}
