package server

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/w99betaCODER/Wisp/internal/model"
	"github.com/w99betaCODER/Wisp/internal/store"
	"github.com/w99betaCODER/Wisp/internal/util"
)

// handleListNodes returns every registered node.
func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.store.ListNodes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list nodes")
		return
	}
	writeJSON(w, http.StatusOK, nodes)
}

// createNodeRequest is the JSON body accepted by POST /api/nodes.
type createNodeRequest struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	Protocol   string `json:"protocol"`
	PublicHost string `json:"public_host"`
	PublicPort int    `json:"public_port"`
}

// handleCreateNode registers a new node agent.
func (s *Server) handleCreateNode(w http.ResponseWriter, r *http.Request) {
	var req createNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == "" || req.Address == "" {
		writeError(w, http.StatusBadRequest, "name and address are required")
		return
	}
	switch req.Protocol {
	case "", "vless":
		req.Protocol = "vless"
	case "vmess", "trojan":
	default:
		writeError(w, http.StatusBadRequest, "protocol must be vless, vmess or trojan")
		return
	}
	if req.PublicPort == 0 {
		req.PublicPort = 443
	}
	if req.PublicHost == "" {
		// Default the VPN host to the agent host (strip the agent port).
		req.PublicHost = req.Address
		if h, _, err := net.SplitHostPort(req.Address); err == nil {
			req.PublicHost = h
		}
	}

	node := model.Node{
		ID:         util.NewID(),
		Name:       req.Name,
		Address:    req.Address,
		Protocol:   req.Protocol,
		PublicHost: req.PublicHost,
		PublicPort: req.PublicPort,
		Enabled:    true,
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.store.CreateNode(node); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create node")
		return
	}
	writeJSON(w, http.StatusCreated, node)
}

// handleGetNode returns a single node by id.
func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	node, err := s.store.GetNode(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get node")
		return
	}
	writeJSON(w, http.StatusOK, node)
}

// handleDeleteNode removes a node by id.
func (s *Server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.store.DeleteNode(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete node")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
