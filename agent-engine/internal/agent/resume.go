package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Agent resume/recovery aligned with claude-code-main's agentResume.ts.
//
// Enables agents to be resumed after interruption by persisting their state
// to disk. On restart, the agent can reload its state and continue execution.

// AgentCheckpoint captures the state of an agent at a point in time.
type AgentCheckpoint struct {
	AgentID      string          `json:"agent_id"`
	SessionID    string          `json:"session_id"`
	Definition   AgentDefinition `json:"definition"`
	Status       AgentStatus     `json:"status"`
	TurnCount    int             `json:"turn_count"`
	MaxTurns     int             `json:"max_turns"`
	WorkDir      string          `json:"work_dir"`
	WorktreeDir  string          `json:"worktree_dir,omitempty"`
	Output       string          `json:"output,omitempty"`
	Error        string          `json:"error,omitempty"`
	ParentID     string          `json:"parent_id,omitempty"`
	TeamName     string          `json:"team_name,omitempty"`
	Background   bool            `json:"background"`
	CreatedAt    time.Time       `json:"created_at"`
	CheckpointAt time.Time       `json:"checkpoint_at"`

	// MessageCount tracks how many messages are in the conversation.
	MessageCount int `json:"message_count"`

	// TranscriptPath is the path to the conversation transcript file
	// used for conversation replay on resume.
	TranscriptPath string `json:"transcript_path,omitempty"`

	// SystemPrompt is the system prompt at checkpoint time.
	SystemPrompt string `json:"system_prompt,omitempty"`

	// Model is the model in use at checkpoint time.
	Model string `json:"model,omitempty"`
}

// ResumeManager handles agent checkpoint persistence and recovery.
type ResumeManager struct {
	baseDir string // directory for checkpoint files
}

// NewResumeManager creates a resume manager that stores checkpoints under baseDir.
func NewResumeManager(baseDir string) *ResumeManager {
	return &ResumeManager{baseDir: baseDir}
}

// checkpointDir returns the directory for agent checkpoints.
func (rm *ResumeManager) checkpointDir() string {
	return filepath.Join(rm.baseDir, ".claude", "agent-checkpoints")
}

// checkpointPath returns the file path for a specific agent's checkpoint.
func (rm *ResumeManager) checkpointPath(agentID string) string {
	return filepath.Join(rm.checkpointDir(), agentID+".json")
}

// SaveCheckpoint persists an agent's state to disk.
func (rm *ResumeManager) SaveCheckpoint(cp *AgentCheckpoint) error {
	dir := rm.checkpointDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create checkpoint dir: %w", err)
	}

	cp.CheckpointAt = time.Now()

	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}

	path := rm.checkpointPath(cp.AgentID)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write checkpoint: %w", err)
	}

	slog.Debug("resume: saved checkpoint",
		slog.String("agent_id", cp.AgentID),
		slog.Int("turn", cp.TurnCount),
	)

	return nil
}

// LoadCheckpoint reads an agent's checkpoint from disk.
func (rm *ResumeManager) LoadCheckpoint(agentID string) (*AgentCheckpoint, error) {
	path := rm.checkpointPath(agentID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no checkpoint
		}
		return nil, fmt.Errorf("read checkpoint: %w", err)
	}

	var cp AgentCheckpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("parse checkpoint: %w", err)
	}

	return &cp, nil
}

// DeleteCheckpoint removes an agent's checkpoint file.
func (rm *ResumeManager) DeleteCheckpoint(agentID string) error {
	path := rm.checkpointPath(agentID)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete checkpoint: %w", err)
	}
	return nil
}

// ListCheckpoints returns all stored agent checkpoints.
func (rm *ResumeManager) ListCheckpoints() ([]*AgentCheckpoint, error) {
	dir := rm.checkpointDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list checkpoints: %w", err)
	}

	var checkpoints []*AgentCheckpoint
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}

		agentID := e.Name()[:len(e.Name())-5] // strip .json
		cp, err := rm.LoadCheckpoint(agentID)
		if err != nil {
			slog.Warn("resume: failed to load checkpoint",
				slog.String("file", e.Name()),
				slog.Any("err", err),
			)
			continue
		}
		if cp != nil {
			checkpoints = append(checkpoints, cp)
		}
	}

	return checkpoints, nil
}

// ListResumable returns checkpoints that can be resumed
// (status is running or pending, not too old).
func (rm *ResumeManager) ListResumable(maxAge time.Duration) ([]*AgentCheckpoint, error) {
	all, err := rm.ListCheckpoints()
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-maxAge)
	var resumable []*AgentCheckpoint
	for _, cp := range all {
		if cp.CheckpointAt.Before(cutoff) {
			continue // too old
		}
		if cp.Status == AgentStatusRunning || cp.Status == AgentStatusPending {
			resumable = append(resumable, cp)
		}
	}

	return resumable, nil
}

// CleanupOld removes checkpoints older than the given duration.
func (rm *ResumeManager) CleanupOld(maxAge time.Duration) (int, error) {
	all, err := rm.ListCheckpoints()
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for _, cp := range all {
		if cp.CheckpointAt.Before(cutoff) {
			if err := rm.DeleteCheckpoint(cp.AgentID); err != nil {
				slog.Warn("resume: cleanup failed",
					slog.String("agent_id", cp.AgentID),
					slog.Any("err", err),
				)
				continue
			}
			removed++
		}
	}

	return removed, nil
}

// BuildResumeParams converts a checkpoint into RunAgentParams for resumption.
func BuildResumeParams(cp *AgentCheckpoint) RunAgentParams {
	params := RunAgentParams{
		AgentDef:        &cp.Definition,
		Task:            cp.Definition.Task,
		WorkDir:         cp.WorkDir,
		MaxTurns:        cp.MaxTurns - cp.TurnCount, // remaining turns
		Background:      cp.Background,
		ExistingAgentID: cp.AgentID,
		Model:           cp.Model,
		SystemPrompt:    cp.SystemPrompt,
		TeamName:        cp.TeamName,
	}

	if cp.WorktreeDir != "" {
		params.WorkDir = cp.WorktreeDir
		params.IsolationMode = IsolationWorktree
	}

	return params
}

// ── Conversation Transcript ────────────────────────────────────────────
// Conversation replay for agent recovery.

// TranscriptEntry is a single message in the conversation transcript.
type TranscriptEntry struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	ToolName  string `json:"tool_name,omitempty"`
	ToolID    string `json:"tool_id,omitempty"`
	Timestamp int64  `json:"ts"`
}

// SaveTranscript writes a conversation transcript to disk alongside the checkpoint.
func (rm *ResumeManager) SaveTranscript(agentID string, entries []TranscriptEntry) (string, error) {
	dir := rm.checkpointDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	path := filepath.Join(dir, agentID+".transcript.json")
	data, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("marshal transcript: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write transcript: %w", err)
	}

	return path, nil
}

// LoadTranscript reads a conversation transcript from disk.
func (rm *ResumeManager) LoadTranscript(agentID string) ([]TranscriptEntry, error) {
	path := filepath.Join(rm.checkpointDir(), agentID+".transcript.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read transcript: %w", err)
	}

	var entries []TranscriptEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse transcript: %w", err)
	}

	return entries, nil
}

// DeleteTranscript removes a conversation transcript file.
func (rm *ResumeManager) DeleteTranscript(agentID string) {
	path := filepath.Join(rm.checkpointDir(), agentID+".transcript.json")
	_ = os.Remove(path)
}

// ── Auto-Checkpoint Hook ───────────────────────────────────────────────
// Called by the runner's executeLoop at regular intervals.

// AutoCheckpointConfig controls automatic checkpointing behavior.
type AutoCheckpointConfig struct {
	Enabled       bool
	IntervalTurns int // checkpoint every N turns (default 5)
	ResumeManager *ResumeManager
}

// ShouldAutoCheckpoint returns true if an auto-checkpoint should be taken.
func (cfg *AutoCheckpointConfig) ShouldAutoCheckpoint(turnCount int) bool {
	if !cfg.Enabled || cfg.ResumeManager == nil {
		return false
	}
	interval := cfg.IntervalTurns
	if interval <= 0 {
		interval = 5
	}
	return turnCount > 0 && turnCount%interval == 0
}

// FormatResumeMessage creates a user-facing message when an agent is resumed.
func FormatResumeMessage(cp *AgentCheckpoint) string {
	age := time.Since(cp.CheckpointAt).Round(time.Second)
	msg := fmt.Sprintf("Resuming agent %s from checkpoint (turn %d/%d, %s ago).",
		truncID(cp.AgentID), cp.TurnCount, cp.MaxTurns, age)
	if cp.TranscriptPath != "" {
		msg += "\nConversation history will be replayed."
	}
	return msg
}
