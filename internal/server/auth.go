package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/w99betaCODER/Wisp/internal/model"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookie = "wisp_session"

// ctxKey is the unexported type for request-context keys, so values stored by
// this package can never collide with keys from another package.
type ctxKey int

const adminCtxKey ctxKey = iota

// syntheticSuper is the identity used when authentication is disabled (no admins
// provisioned) or when a request authenticates with the API token. It owns
// nothing, but being super it sees and manages everything.
var syntheticSuper = model.Admin{Username: "root", Role: model.RoleSuper}

// authEnabled reports whether any admin exists. With none provisioned the panel
// runs open (dev mode): every request acts as the super-admin.
func (s *Server) authEnabled() bool {
	admins, err := s.store.ListAdmins()
	return err == nil && len(admins) > 0
}

// signSession returns a tamper-proof cookie value binding the admin id with an
// HMAC-SHA256 tag. It needs no server-side session store and cannot be forged
// without the session secret.
func (s *Server) signSession(adminID string) string {
	mac := hmac.New(sha256.New, s.sessionSecret)
	mac.Write([]byte(adminID))
	return adminID + "." + hex.EncodeToString(mac.Sum(nil))
}

// verifySession validates a cookie value and returns the admin id it carries.
func (s *Server) verifySession(token string) (string, bool) {
	i := strings.LastIndex(token, ".")
	if i < 0 {
		return "", false
	}
	id, sig := token[:i], token[i+1:]
	mac := hmac.New(sha256.New, s.sessionSecret)
	mac.Write([]byte(id))
	want := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(sig), []byte(want)) != 1 {
		return "", false
	}
	return id, true
}

// auth resolves the calling admin and stashes it in the request context. When a
// caller cannot be identified, public paths still pass through (login, branding,
// the HMAC-signed webhook, the dashboard, /sub) while everything else gets 401.
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if admin, ok := s.currentAdmin(r); ok {
			r = r.WithContext(context.WithValue(r.Context(), adminCtxKey, admin))
			next.ServeHTTP(w, r)
			return
		}
		if !protected(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, http.StatusUnauthorized, "authentication required")
	})
}

// currentAdmin identifies the caller from (in order) the session cookie, the
// API bearer token, or — when auth is disabled — the synthetic super-admin.
func (s *Server) currentAdmin(r *http.Request) (model.Admin, bool) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if id, ok := s.verifySession(c.Value); ok {
			if a, err := s.store.GetAdmin(id); err == nil {
				return a, true
			}
		}
	}
	if s.cfg.APIToken != "" {
		h := r.Header.Get("Authorization")
		if strings.HasPrefix(h, "Bearer ") &&
			subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(h, "Bearer ")), []byte(s.cfg.APIToken)) == 1 {
			return syntheticSuper, true
		}
	}
	if !s.authEnabled() {
		return syntheticSuper, true
	}
	return model.Admin{}, false
}

// protected reports whether a path requires an identified caller.
func protected(path string) bool {
	if !strings.HasPrefix(path, "/api/") {
		return false // dashboard, /sub, /healthz, static assets
	}
	switch {
	case path == "/api/login",
		path == "/api/logout",
		path == "/api/branding",
		strings.HasPrefix(path, "/api/webhook/"): // webhook is HMAC-authenticated
		return false
	}
	return true
}

// adminFrom returns the admin attached to the request by the auth middleware.
// On protected routes this is always populated; elsewhere it may be the zero
// Admin.
func adminFrom(r *http.Request) model.Admin {
	a, _ := r.Context().Value(adminCtxKey).(model.Admin)
	return a
}

// requireSuper writes 403 and returns false unless the caller is a super-admin.
func (s *Server) requireSuper(w http.ResponseWriter, r *http.Request) bool {
	if adminFrom(r).Role == model.RoleSuper {
		return true
	}
	writeError(w, http.StatusForbidden, "super-admin only")
	return false
}

// canAccessUser reports whether the caller may view or mutate u. Super-admins
// may touch everyone; resellers only the users they own.
func canAccessUser(r *http.Request, u model.User) bool {
	a := adminFrom(r)
	return a.Role == model.RoleSuper || u.OwnerID == a.ID
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleLogin verifies username/password against the admin table (bcrypt) and
// sets a signed session cookie.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !s.authEnabled() {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "role": model.RoleSuper}) // login disabled
		return
	}
	admin, err := s.store.GetAdminByUsername(req.Username)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    s.signSession(admin.ID),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "username": admin.Username, "role": admin.Role})
}

// handleLogout clears the session cookie.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleMe returns the signed-in admin's identity, driving the role-aware UI.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	a := adminFrom(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"username":     a.Username,
		"role":         a.Role,
		"auth_enabled": s.authEnabled(),
	})
}

// handleBranding returns the white-label settings the dashboard applies.
func (s *Server) handleBranding(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cfg.Brand)
}
