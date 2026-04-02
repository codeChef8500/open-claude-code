package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/wall-ai/agent-engine/internal/service/mcp"
	"github.com/wall-ai/agent-engine/internal/session"
)

// ── Global MCP manager (process-scoped singleton) ─────────────────────────────

var globalMCPManager = mcp.NewManager()

// ── POST /api/v1/sessions/{id}/fork ──────────────────────────────────────────

type forkSessionRequest struct {
	NewSessionID     string `json:"new_session_id"`
	FromMessageIndex int    `json:"from_message_index,omitempty"`
	Label            string `json:"label,omitempty"`
}

type forkSessionResponse struct {
	NewSessionID string `json:"new_session_id"`
}

func handleForkSession(w http.ResponseWriter, r *http.Request) {
	srcID := chi.URLParam(r, "id")
	var req forkSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.NewSessionID == "" {
		writeError(w, http.StatusBadRequest, "new_session_id is required")
		return
	}

	store := session.NewStorage(session.DefaultStorageDir())
	fromIdx := -1
	if req.FromMessageIndex > 0 {
		fromIdx = req.FromMessageIndex
	}
	if err := store.Fork(srcID, session.ForkOptions{
		NewSessionID:     req.NewSessionID,
		FromMessageIndex: fromIdx,
		Label:            req.Label,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("fork session: %v", err))
		return
	}
	writeJSON(w, http.StatusCreated, forkSessionResponse{NewSessionID: req.NewSessionID})
}

// ── GET /api/v1/mcp/servers ───────────────────────────────────────────────────

type mcpServerInfo struct {
	Name  string   `json:"name"`
	Tools []string `json:"tools"`
}

func handleListMCPServers(w http.ResponseWriter, _ *http.Request) {
	tools := globalMCPManager.AllTools()

	// Group tool names by server.
	byServer := make(map[string][]string)
	for _, t := range tools {
		byServer[t.ServerName] = append(byServer[t.ServerName], t.Tool.Name)
	}

	result := make([]mcpServerInfo, 0, len(byServer))
	for name, toolNames := range byServer {
		result = append(result, mcpServerInfo{Name: name, Tools: toolNames})
	}
	writeJSON(w, http.StatusOK, result)
}

// ── POST /api/v1/mcp/servers ──────────────────────────────────────────────────

type connectMCPRequest struct {
	Name      string   `json:"name"`
	Command   string   `json:"command"`
	Args      []string `json:"args,omitempty"`
	Env       []string `json:"env,omitempty"`
	Transport string   `json:"transport,omitempty"`
	URL       string   `json:"url,omitempty"`
}

func handleConnectMCPServer(w http.ResponseWriter, r *http.Request) {
	var req connectMCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	cfg := mcp.ServerConfig{
		Name:      req.Name,
		Command:   req.Command,
		Args:      req.Args,
		Env:       req.Env,
		Transport: req.Transport,
		URL:       req.URL,
	}
	if err := globalMCPManager.Connect(r.Context(), cfg); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("connect mcp server: %v", err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "connected", "name": req.Name})
}

// ── DELETE /api/v1/mcp/servers/{name} ────────────────────────────────────────

func handleDisconnectMCPServer(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := globalMCPManager.Disconnect(name); err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("disconnect mcp server: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disconnected", "name": name})
}
