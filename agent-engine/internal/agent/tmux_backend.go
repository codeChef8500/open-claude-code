package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Tmux backend aligned with claude-code-main's tmuxTeammate.ts.
//
// The tmux backend runs teammate agents in separate tmux panes/windows,
// providing process-level isolation while keeping agents on the same machine.
// Each agent gets its own tmux pane where it runs the agent-engine binary.

// TmuxSessionState tracks the state of a tmux-backed agent.
type TmuxSessionState string

const (
	TmuxStateCreating TmuxSessionState = "creating"
	TmuxStateRunning  TmuxSessionState = "running"
	TmuxStateStopped  TmuxSessionState = "stopped"
	TmuxStateFailed   TmuxSessionState = "failed"
)

// TmuxTeammate represents an agent running in a tmux pane.
type TmuxTeammate struct {
	mu sync.RWMutex

	AgentID     string
	SessionName string
	WindowName  string
	PaneID      string
	State       TmuxSessionState
	StartedAt   time.Time
	StoppedAt   time.Time
	WorkDir     string
	Error       error
}

// TmuxBackend manages tmux-based teammate agents.
type TmuxBackend struct {
	mu         sync.RWMutex
	teammates  map[string]*TmuxTeammate
	sessionBase string // base tmux session name
	binary      string // path to agent-engine binary
}

// TmuxBackendConfig configures the tmux backend.
type TmuxBackendConfig struct {
	SessionBase string // base session name (default: "agent-swarm")
	Binary      string // path to agent-engine binary
}

// NewTmuxBackend creates a new tmux backend.
func NewTmuxBackend(cfg TmuxBackendConfig) *TmuxBackend {
	if cfg.SessionBase == "" {
		cfg.SessionBase = "agent-swarm"
	}
	if cfg.Binary == "" {
		cfg.Binary = "agent-engine"
	}
	return &TmuxBackend{
		teammates:   make(map[string]*TmuxTeammate),
		sessionBase: cfg.SessionBase,
		binary:      cfg.Binary,
	}
}

// IsTmuxAvailable checks if tmux is installed and accessible.
func IsTmuxAvailable() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// EnsureSession creates the base tmux session if it doesn't exist.
func (tb *TmuxBackend) EnsureSession() error {
	// Check if session exists.
	cmd := exec.Command("tmux", "has-session", "-t", tb.sessionBase)
	if err := cmd.Run(); err == nil {
		return nil // session exists
	}

	// Create new detached session.
	cmd = exec.Command("tmux", "new-session", "-d", "-s", tb.sessionBase)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux new-session: %w\n%s", err, out)
	}

	slog.Info("tmux: created session", slog.String("session", tb.sessionBase))
	return nil
}

// SpawnTeammate creates a new tmux window/pane for an agent.
func (tb *TmuxBackend) SpawnTeammate(ctx context.Context, agentID, task, workDir string) (*TmuxTeammate, error) {
	if !IsTmuxAvailable() {
		return nil, fmt.Errorf("tmux is not installed")
	}

	if err := tb.EnsureSession(); err != nil {
		return nil, err
	}

	windowName := fmt.Sprintf("agent-%s", agentID[:8])

	teammate := &TmuxTeammate{
		AgentID:     agentID,
		SessionName: tb.sessionBase,
		WindowName:  windowName,
		State:       TmuxStateCreating,
		StartedAt:   time.Now(),
		WorkDir:     workDir,
	}

	tb.mu.Lock()
	tb.teammates[agentID] = teammate
	tb.mu.Unlock()

	// Create a new tmux window with the agent command.
	agentCmd := tb.buildAgentCommand(agentID, task, workDir)
	cmd := exec.CommandContext(ctx, "tmux", "new-window",
		"-t", tb.sessionBase,
		"-n", windowName,
		agentCmd,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		teammate.mu.Lock()
		teammate.State = TmuxStateFailed
		teammate.Error = fmt.Errorf("tmux new-window: %w\n%s", err, out)
		teammate.mu.Unlock()
		return teammate, teammate.Error
	}

	// Capture the pane ID.
	paneCmd := exec.Command("tmux", "display-message", "-p",
		"-t", fmt.Sprintf("%s:%s", tb.sessionBase, windowName),
		"#{pane_id}")
	paneOut, err := paneCmd.Output()
	if err == nil {
		teammate.PaneID = strings.TrimSpace(string(paneOut))
	}

	teammate.mu.Lock()
	teammate.State = TmuxStateRunning
	teammate.mu.Unlock()

	slog.Info("tmux: spawned teammate",
		slog.String("agent_id", agentID),
		slog.String("window", windowName),
		slog.String("pane_id", teammate.PaneID),
	)

	return teammate, nil
}

// StopTeammate kills the tmux window for an agent.
func (tb *TmuxBackend) StopTeammate(agentID string) error {
	tb.mu.RLock()
	teammate, ok := tb.teammates[agentID]
	tb.mu.RUnlock()

	if !ok {
		return fmt.Errorf("tmux teammate not found: %s", agentID)
	}

	target := fmt.Sprintf("%s:%s", tb.sessionBase, teammate.WindowName)
	cmd := exec.Command("tmux", "kill-window", "-t", target)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux kill-window: %w\n%s", err, out)
	}

	teammate.mu.Lock()
	teammate.State = TmuxStateStopped
	teammate.StoppedAt = time.Now()
	teammate.mu.Unlock()

	slog.Info("tmux: stopped teammate", slog.String("agent_id", agentID))
	return nil
}

// IsAlive checks if the tmux pane for an agent is still running.
func (tb *TmuxBackend) IsAlive(agentID string) bool {
	tb.mu.RLock()
	teammate, ok := tb.teammates[agentID]
	tb.mu.RUnlock()

	if !ok || teammate.PaneID == "" {
		return false
	}

	cmd := exec.Command("tmux", "list-panes", "-a", "-F", "#{pane_id}")
	out, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.Contains(string(out), teammate.PaneID)
}

// CaptureOutput captures the current output of a tmux pane.
func (tb *TmuxBackend) CaptureOutput(agentID string, lines int) (string, error) {
	tb.mu.RLock()
	teammate, ok := tb.teammates[agentID]
	tb.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("tmux teammate not found: %s", agentID)
	}

	if lines <= 0 {
		lines = 100
	}

	target := fmt.Sprintf("%s:%s", tb.sessionBase, teammate.WindowName)
	cmd := exec.Command("tmux", "capture-pane", "-p",
		"-t", target,
		"-S", fmt.Sprintf("-%d", lines))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tmux capture-pane: %w", err)
	}

	return string(out), nil
}

// SendKeys sends keystrokes to a tmux pane (for stdin interaction).
func (tb *TmuxBackend) SendKeys(agentID, keys string) error {
	tb.mu.RLock()
	teammate, ok := tb.teammates[agentID]
	tb.mu.RUnlock()

	if !ok {
		return fmt.Errorf("tmux teammate not found: %s", agentID)
	}

	target := fmt.Sprintf("%s:%s", tb.sessionBase, teammate.WindowName)
	cmd := exec.Command("tmux", "send-keys", "-t", target, keys, "Enter")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux send-keys: %w\n%s", err, out)
	}
	return nil
}

// AllTeammates returns all tmux teammates.
func (tb *TmuxBackend) AllTeammates() []*TmuxTeammate {
	tb.mu.RLock()
	defer tb.mu.RUnlock()
	result := make([]*TmuxTeammate, 0, len(tb.teammates))
	for _, t := range tb.teammates {
		result = append(result, t)
	}
	return result
}

// StopAll kills all tmux teammate windows.
func (tb *TmuxBackend) StopAll() {
	tb.mu.RLock()
	ids := make([]string, 0, len(tb.teammates))
	for id := range tb.teammates {
		ids = append(ids, id)
	}
	tb.mu.RUnlock()

	for _, id := range ids {
		_ = tb.StopTeammate(id)
	}
}

// DestroySession kills the entire tmux session.
func (tb *TmuxBackend) DestroySession() error {
	cmd := exec.Command("tmux", "kill-session", "-t", tb.sessionBase)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux kill-session: %w\n%s", err, out)
	}
	slog.Info("tmux: destroyed session", slog.String("session", tb.sessionBase))
	return nil
}

// buildAgentCommand constructs the shell command to run an agent in tmux.
func (tb *TmuxBackend) buildAgentCommand(agentID, task, workDir string) string {
	// Escape task for shell.
	escapedTask := strings.ReplaceAll(task, "'", "'\\''")
	return fmt.Sprintf("cd '%s' && '%s' run --agent-id='%s' --task='%s'",
		workDir, tb.binary, agentID, escapedTask)
}
