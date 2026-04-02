package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	agentcoord "github.com/wall-ai/agent-engine/internal/agent"
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/util"
	"github.com/wall-ai/agent-engine/pkg/sdk"
)

var (
	enginesMu sync.RWMutex
	engines   = make(map[string]*sdk.Engine)

	coordinatorOnce sync.Once
	globalCoord     *agentcoord.Coordinator
)

func getCoordinator() *agentcoord.Coordinator {
	coordinatorOnce.Do(func() {
		globalCoord = agentcoord.NewCoordinator(nil, nil)
	})
	return globalCoord
}

func registerRoutes(r *chi.Mux) {
	r.Get("/health", handleHealth)

	r.Route("/api/v1", func(r chi.Router) {
		// Sessions
		r.Post("/sessions", handleCreateSession)
		r.Get("/sessions", handleListSessions)
		r.Get("/sessions/{id}", handleGetSession)
		r.Post("/sessions/{id}/messages", handleSendMessage)
		r.Delete("/sessions/{id}", handleDeleteSession)
		r.Get("/sessions/{id}/export", handleExportSession)
		r.Post("/sessions/{id}/fork", handleForkSession)

		// MCP
		r.Get("/mcp/servers", handleListMCPServers)
		r.Post("/mcp/servers", handleConnectMCPServer)
		r.Delete("/mcp/servers/{name}", handleDisconnectMCPServer)

		// Tools
		r.Get("/tools", handleListTools)

		// Memory
		r.Get("/sessions/{id}/memory", handleGetMemory)

		// Commands
		r.Post("/sessions/{id}/commands", handleRunCommand)

		// Agents
		r.Post("/sessions/{id}/agents", handleSpawnAgent)
		r.Get("/sessions/{id}/agents", handleListAgents)
		r.Delete("/sessions/{id}/agents/{agentId}", handleCancelAgent)

		// Skills
		r.Get("/skills", handleListSkills)

		// Plugins
		r.Get("/plugins", handleListPlugins)
		r.Post("/plugins", handleLoadPlugin)
		r.Delete("/plugins/{name}", handleUnloadPlugin)

		// Session search + stats
		r.Get("/sessions/{id}/search", handleSearchSession)
		r.Get("/sessions/{id}/stats", handleSessionStats)
		r.Get("/search", handleSearchAllSessions)

		// Config
		r.Get("/config", handleGetConfig)
		r.Put("/config", handleUpdateConfig)

		// Buddy
		r.Get("/buddy", handleGetBuddy)

		// Modes
		r.Get("/modes", handleGetModes)
		r.Post("/modes", handleSetMode)
	})
}

// ─── /health ──────────────────────────────────────────────────────────────────

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// ─── POST /api/v1/sessions ────────────────────────────────────────────────────

type createSessionRequest struct {
	WorkDir            string `json:"work_dir"`
	Provider           string `json:"provider,omitempty"`
	Model              string `json:"model,omitempty"`
	APIKey             string `json:"api_key,omitempty"`
	BaseURL            string `json:"base_url,omitempty"`
	MaxTokens          int    `json:"max_tokens,omitempty"`
	ThinkingBudget     int    `json:"thinking_budget,omitempty"`
	CustomSystemPrompt string `json:"custom_system_prompt,omitempty"`
	AppendSystemPrompt string `json:"append_system_prompt,omitempty"`
	AutoMode           bool   `json:"auto_mode,omitempty"`
	Verbose            bool   `json:"verbose,omitempty"`
}

type createSessionResponse struct {
	SessionID string `json:"session_id"`
}

func handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.WorkDir == "" {
		writeError(w, http.StatusBadRequest, "work_dir is required")
		return
	}

	// Resolve API key: request body > env var
	apiKey := req.APIKey
	if apiKey == "" {
		apiKey = util.GetString("api_key")
	}
	if apiKey == "" && (req.Provider == "" || req.Provider == "anthropic") {
		apiKey = util.EnvString("ANTHROPIC_API_KEY", "")
	}
	if apiKey == "" && req.Provider == "openai" {
		apiKey = util.EnvString("OPENAI_API_KEY", "")
	}

	opts := []sdk.Option{
		sdk.WithWorkDir(req.WorkDir),
		sdk.WithAPIKey(apiKey),
		sdk.WithAutoMode(req.AutoMode),
		sdk.WithVerbose(req.Verbose),
	}
	if req.Provider != "" {
		opts = append(opts, sdk.WithProvider(req.Provider))
	}
	if req.Model != "" {
		opts = append(opts, sdk.WithModel(req.Model))
	}
	if req.MaxTokens > 0 {
		opts = append(opts, sdk.WithMaxTokens(req.MaxTokens))
	}
	if req.ThinkingBudget > 0 {
		opts = append(opts, sdk.WithThinkingBudget(req.ThinkingBudget))
	}
	if req.BaseURL != "" {
		opts = append(opts, sdk.WithBaseURL(req.BaseURL))
	}
	if req.CustomSystemPrompt != "" {
		opts = append(opts, sdk.WithCustomSystemPrompt(req.CustomSystemPrompt))
	}
	if req.AppendSystemPrompt != "" {
		opts = append(opts, sdk.WithAppendSystemPrompt(req.AppendSystemPrompt))
	}

	eng, err := sdk.New(opts...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	enginesMu.Lock()
	engines[eng.SessionID()] = eng
	enginesMu.Unlock()

	writeJSON(w, http.StatusCreated, createSessionResponse{SessionID: eng.SessionID()})
}

// ─── GET /api/v1/sessions/{id} ────────────────────────────────────────────────

func handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	enginesMu.RLock()
	eng, ok := engines[id]
	enginesMu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("session %q not found", id))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"session_id": eng.SessionID()})
}

// ─── POST /api/v1/sessions/{id}/messages ─────────────────────────────────────

type sendMessageRequest struct {
	Text   string   `json:"text"`
	Images []string `json:"images,omitempty"`
	Stream bool     `json:"stream,omitempty"`
}

func handleSendMessage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	enginesMu.RLock()
	eng, ok := engines[id]
	enginesMu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("session %q not found", id))
		return
	}

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Text == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}

	ctx := r.Context()

	if req.Stream {
		// Server-Sent Events streaming response
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "streaming not supported")
			return
		}

		eventCh := eng.SubmitMessage(ctx, req.Text)
		for ev := range eventCh {
			b, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
		return
	}

	// Non-streaming: collect full response
	eventCh := eng.SubmitMessage(ctx, req.Text)
	var text string
	var usage *engine.UsageStats
	var errMsg string

	for ev := range eventCh {
		switch ev.Type {
		case engine.EventTextDelta:
			text += ev.Text
		case engine.EventUsage:
			usage = ev.Usage
		case engine.EventError:
			errMsg = ev.Error
		}
	}

	if errMsg != "" {
		writeError(w, http.StatusInternalServerError, errMsg)
		return
	}

	resp := map[string]interface{}{
		"text": text,
	}
	if usage != nil {
		resp["usage"] = usage
	}
	writeJSON(w, http.StatusOK, resp)
}

// ─── DELETE /api/v1/sessions/{id} ────────────────────────────────────────────

func handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	enginesMu.Lock()
	eng, ok := engines[id]
	if ok {
		_ = eng.Close()
		delete(engines, id)
	}
	enginesMu.Unlock()

	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("session %q not found", id))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
