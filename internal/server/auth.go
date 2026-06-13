package server

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
)

const authCookie = "wisp_token"

// auth gates the admin API behind the configured token. Public paths — the
// dashboard, subscriptions, health, login, branding and the signed webhook —
// are always allowed. When no token is configured, auth is a pass-through.
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.APIToken == "" || !protected(r.URL.Path) || s.validToken(r) {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, http.StatusUnauthorized, "authentication required")
	})
}

// protected reports whether a path requires the API token.
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

// validToken checks the Bearer header or the session cookie in constant time.
func (s *Server) validToken(r *http.Request) bool {
	tok := ""
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		tok = strings.TrimPrefix(h, "Bearer ")
	} else if c, err := r.Cookie(authCookie); err == nil {
		tok = c.Value
	}
	return tok != "" && subtle.ConstantTimeCompare([]byte(tok), []byte(s.cfg.APIToken)) == 1
}

type loginRequest struct {
	Token string `json:"token"`
}

// handleLogin sets an httpOnly session cookie when the token matches.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if s.cfg.APIToken == "" {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true}) // auth disabled
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.Token), []byte(s.cfg.APIToken)) != 1 {
		writeError(w, http.StatusUnauthorized, "invalid token")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authCookie,
		Value:    req.Token,
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
