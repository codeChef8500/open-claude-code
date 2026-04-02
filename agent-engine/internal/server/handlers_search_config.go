package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/wall-ai/agent-engine/internal/session"
	"github.com/wall-ai/agent-engine/internal/util"
)

// ─── GET /api/v1/sessions/{id}/search ────────────────────────────────────────

func handleSearchSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	storage := session.NewStorage(session.DefaultStorageDir())
	results, err := storage.SearchTranscript(id, query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session_id": id,
		"query":      query,
		"results":    results,
		"count":      len(results),
	})
}

// ─── GET /api/v1/sessions/{id}/stats ─────────────────────────────────────────

func handleSessionStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	storage := session.NewStorage(session.DefaultStorageDir())

	stats, err := storage.ComputeStats(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	meta, _ := storage.LoadMeta(id)

	resp := map[string]interface{}{
		"session_id": id,
		"stats":      stats,
	}
	if meta != nil {
		resp["meta"] = meta
	}
	writeJSON(w, http.StatusOK, resp)
}

// ─── GET /api/v1/search ──────────────────────────────────────────────────────

func handleSearchAllSessions(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}
	maxStr := r.URL.Query().Get("max")
	max := 50
	if maxStr != "" {
		if n, err := strconv.Atoi(maxStr); err == nil && n > 0 {
			max = n
		}
	}

	storage := session.NewStorage(session.DefaultStorageDir())
	results, err := storage.SearchAllSessions(query, max)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"query":   query,
		"results": results,
		"count":   len(results),
	})
}

// ─── GET /api/v1/config ──────────────────────────────────────────────────────

func handleGetConfig(w http.ResponseWriter, r *http.Request) {
	workDir := r.URL.Query().Get("work_dir")
	if workDir == "" {
		workDir = "."
	}

	cfg, err := util.LoadAppConfig(workDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("load config: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"config":  cfg,
		"summary": cfg.Summary(),
	})
}

// ─── PUT /api/v1/config ──────────────────────────────────────────────────────

type updateConfigRequest struct {
	WorkDir string                 `json:"work_dir"`
	Updates map[string]interface{} `json:"updates"`
}

func handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req updateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	workDir := req.WorkDir
	if workDir == "" {
		workDir = "."
	}

	cfg, err := util.LoadAppConfig(workDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("load config: %v", err))
		return
	}

	// Apply updates.
	for k, v := range req.Updates {
		cfg.Set(k, v)
	}

	if err := cfg.SaveToProject(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("save config: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "updated",
		"config": cfg,
	})
}
