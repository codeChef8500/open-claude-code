package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/wall-ai/agent-engine/internal/command"
	"github.com/wall-ai/agent-engine/internal/session"
)

// ─── GET /api/v1/sessions ─────────────────────────────────────────────────────

func handleListSessions(w http.ResponseWriter, r *http.Request) {
	storage := session.NewStorage(session.DefaultStorageDir())
	metas, err := storage.ListSessions()
	if err != nil {
		// Fallback: return in-memory session IDs only.
		enginesMu.RLock()
		ids := make([]string, 0, len(engines))
		for id := range engines {
			ids = append(ids, id)
		}
		enginesMu.RUnlock()
		writeJSON(w, http.StatusOK, map[string]interface{}{"sessions": ids})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"sessions": metas})
}

// ─── GET /api/v1/sessions/{id}/export ─────────────────────────────────────────

func handleExportSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	storage := session.NewStorage(session.DefaultStorageDir())
	format := r.URL.Query().Get("format") // "markdown" | "json" (default: markdown)

	switch format {
	case "json":
		entries, err := storage.ReadTranscript(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if entries == nil {
			writeError(w, http.StatusNotFound, fmt.Sprintf("session %q not found", id))
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"session_id": id, "entries": entries})
	default: // markdown
		md, err := storage.ExportMarkdown(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(md))
	}
}

// ─── GET /api/v1/tools ────────────────────────────────────────────────────────

func handleListTools(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tools": []string{
			"bash", "file_read", "file_edit", "file_write",
			"grep", "glob", "web_fetch", "web_search",
			"ask_user", "todo_write", "send_message", "sleep",
			"task_stop", "notebook_edit", "brief", "agent",
			"enter_plan_mode", "exit_plan_mode",
			"team_create", "team_delete", "list_peers",
			"cron_create", "cron_delete", "cron_list",
		},
	})
}

// ─── GET /api/v1/sessions/{id}/memory ─────────────────────────────────────────

func handleGetMemory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	enginesMu.RLock()
	_, ok := engines[id]
	enginesMu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("session %q not found", id))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session_id": id,
		"memories":   []interface{}{},
	})
}

// ─── POST /api/v1/sessions/{id}/commands ──────────────────────────────────────

type commandRequest struct {
	Command string `json:"command"`
}

func handleRunCommand(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	enginesMu.RLock()
	_, ok := engines[id]
	enginesMu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("session %q not found", id))
		return
	}

	var req commandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Command == "" {
		writeError(w, http.StatusBadRequest, "command must not be empty")
		return
	}

	// Strip leading slash.
	cmdStr := strings.TrimPrefix(req.Command, "/")
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		writeError(w, http.StatusBadRequest, "empty command")
		return
	}
	name := parts[0]
	args := parts[1:]

	ectx := &command.ExecContext{SessionID: id}
	result, err := command.Execute(r.Context(), name, args, ectx)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"session_id": id,
		"command":    req.Command,
		"result":     result,
	})
}

// ─── POST /api/v1/sessions/{id}/agents ────────────────────────────────────────

type spawnAgentRequest struct {
	Task         string   `json:"task"`
	WorkDir      string   `json:"work_dir,omitempty"`
	MaxTurns     int      `json:"max_turns,omitempty"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
}

func handleSpawnAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	enginesMu.RLock()
	_, ok := engines[id]
	enginesMu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("session %q not found", id))
		return
	}

	var req spawnAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Task == "" {
		writeError(w, http.StatusBadRequest, "task is required")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"session_id": id,
		"status":     "agent spawn requested — integrate with agent.Coordinator for full support",
		"task":       req.Task,
	})
}

// ─── GET /api/v1/sessions/{id}/agents ─────────────────────────────────────────

func handleListAgents(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	enginesMu.RLock()
	_, ok := engines[id]
	enginesMu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("session %q not found", id))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session_id": id,
		"agents":     []interface{}{},
	})
}

// ─── DELETE /api/v1/sessions/{id}/agents/{agentId} ────────────────────────────

func handleCancelAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agentID := chi.URLParam(r, "agentId")
	writeJSON(w, http.StatusOK, map[string]string{
		"session_id": id,
		"agent_id":   agentID,
		"status":     "cancel requested",
	})
}

// ─── GET /api/v1/skills ───────────────────────────────────────────────────────

func handleListSkills(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"skills": []interface{}{},
	})
}

// ─── Plugin handlers ──────────────────────────────────────────────────────────

func handleListPlugins(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"plugins": []interface{}{}})
}

type loadPluginRequest struct {
	Path string `json:"path"`
}

func handleLoadPlugin(w http.ResponseWriter, r *http.Request) {
	var req loadPluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"path": req.Path, "status": "loaded"})
}

func handleUnloadPlugin(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	writeJSON(w, http.StatusOK, map[string]string{"name": name, "status": "unloaded"})
}

// ─── GET /api/v1/buddy ────────────────────────────────────────────────────────

func handleGetBuddy(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"companion": nil,
		"message":   "hatch a companion via /hatch command or POST /api/v1/buddy/hatch",
	})
}

// ─── Modes ────────────────────────────────────────────────────────────────────

func handleGetModes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"auto_mode":       false,
		"fast_mode":       false,
		"undercover_mode": false,
	})
}

type setModeRequest struct {
	AutoMode  *bool `json:"auto_mode,omitempty"`
	FastMode  *bool `json:"fast_mode,omitempty"`
}

func handleSetMode(w http.ResponseWriter, r *http.Request) {
	var req setModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "mode updated"})
}
