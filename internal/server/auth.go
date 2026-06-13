package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

const sessionCookie = "wisp_session"

// authEnabled reports whether any authentication is configured.
func (s *Server) authEnabled() bool {
	return s.cfg.AdminPass != "" || s.cfg.APIToken != ""
}

// sessionValue is a stable secret derived from the admin credentials and stored
// in the session cookie. It needs no server-side session store and cannot be
// forged without knowing the password.
func (s *Server) sessionValue() string {
	sum := sha256.Sum256([]byte(s.cfg.AdminUser + ":" + s.cfg.AdminPass))
	return hex.EncodeToString(sum[:])
}

// auth gates the admin API. Public paths — dashboard, /sub, health, login,
// branding and the HMAC-signed webhook — are always allowed. When nothing is
// configured, auth is a pass-through.
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authEnabled() || !protected(r.URL.Path) || s.authorized(r) {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, http.StatusUnauthorized, "authentication required")
	})
}

// protected reports whether a path requires authentication.
func protected(path string) bool {
	if !strings.HasPrefix(path, "/api/") {
		return false // dashboard, /sub, /healthz, static assets
	}
	switch {
	case path == "/api/login",
		path == "/api/branding",
		strings.HasPrefix(path, "/api/webhook/"): // webhook is HMAC-authenticated
		return false
	}
	return true
}

// authorized accepts a valid session cookie (dashboard login) or a Bearer token
// equal to WISP_API_TOKEN (scripts / API clients).
func (s *Server) authorized(r *http.Request) bool {
	if s.cfg.AdminPass != "" {
		if c, err := r.Cookie(sessionCookie); err == nil &&
			subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.sessionValue())) == 1 {
			return true
		}
	}
	if s.cfg.APIToken != "" {
		h := r.Header.Get("Authorization")
		if strings.HasPrefix(h, "Bearer ") &&
			subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(h, "Bearer ")), []byte(s.cfg.APIToken)) == 1 {
			return true
		}
	}
	return false
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleLogin verifies the username/password and sets the session cookie.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if s.cfg.AdminPass == "" {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true}) // login disabled
		return
	}
	userOK := subtle.ConstantTimeCompare([]byte(req.Username), []byte(s.cfg.AdminUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.cfg.AdminPass)) == 1
	if !userOK || !passOK {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    s.sessionValue(),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleBranding returns the white-label settings the dashboard applies.
func (s *Server) handleBranding(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cfg.Brand)
}
