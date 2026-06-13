// Package agent is the Wisp node agent: a small HTTP server that runs on each
// VPN server and applies user changes to the local Xray instance. The panel
// reaches it over mTLS; the agent talks to Xray over the local gRPC API.
package agent

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/w99betaCODER/Wisp/internal/xray"
)

// Agent applies user operations to a single local Xray instance.
type Agent struct {
	xray       xray.Client
	inboundTag string
}

// New constructs an Agent.
func New(xc xray.Client, inboundTag string) *Agent {
	return &Agent{xray: xc, inboundTag: inboundTag}
}

// Routes returns the agent's HTTP handler.
func (a *Agent) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.handleHealth)
	mux.HandleFunc("POST /v1/users", a.handleAddUser)
	mux.HandleFunc("DELETE /v1/users/{email}", a.handleRemoveUser)
	return mux
}

func (a *Agent) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// addUserRequest is the body of POST /v1/users.
type addUserRequest struct {
	Email string `json:"email"`
	UUID  string `json:"uuid"`
	Flow  string `json:"flow"`
}

func (a *Agent) handleAddUser(w http.ResponseWriter, r *http.Request) {
	var req addUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Email == "" || req.UUID == "" {
		writeError(w, http.StatusBadRequest, "email and uuid are required")
		return
	}
	if err := a.xray.AddUser(r.Context(), a.inboundTag, req.Email, req.UUID, req.Flow); err != nil {
		log.Printf("agent: add user %q: %v", req.Email, err)
		writeError(w, http.StatusBadGateway, "xray add failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Agent) handleRemoveUser(w http.ResponseWriter, r *http.Request) {
	email := r.PathValue("email")
	if err := a.xray.RemoveUser(r.Context(), a.inboundTag, email); err != nil {
		log.Printf("agent: remove user %q: %v", email, err)
		writeError(w, http.StatusBadGateway, "xray remove failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeJSON / writeError are small local responders; the agent is a separate
// binary, so it does not share the panel's server package helpers.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("agent: writeJSON: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
