package agent

import (
	"fmt"
	"sync"
	"time"
)

// Progress tracking for agent execution, aligned with claude-code-main's
// progress tracking in asyncAgentLifecycle.ts.

// ProgressState represents the current progress of an agent.
type ProgressState struct {
	AgentID     string    `json:"agent_id"`
	TurnCount   int       `json:"turn_count"`
	MaxTurns    int       `json:"max_turns"`
	LastTool    string    `json:"last_tool,omitempty"`
	LastMessage string    `json:"last_message,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ProgressTracker tracks the incremental progress of an agent execution.
type ProgressTracker struct {
	mu    sync.RWMutex
	state ProgressState
}

// NewProgressTracker creates a new ProgressTracker for an agent.
func NewProgressTracker(agentID string) *ProgressTracker {
	return &ProgressTracker{
		state: ProgressState{
			AgentID:   agentID,
			UpdatedAt: time.Now(),
		},
	}
}

// SetMaxTurns sets the maximum number of turns.
func (pt *ProgressTracker) SetMaxTurns(max int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.state.MaxTurns = max
}

// IncrementTurn increments the turn count and updates the timestamp.
func (pt *ProgressTracker) IncrementTurn() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.state.TurnCount++
	pt.state.UpdatedAt = time.Now()
}

// SetLastTool records the most recently used tool.
func (pt *ProgressTracker) SetLastTool(toolName string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.state.LastTool = toolName
	pt.state.UpdatedAt = time.Now()
}

// SetLastMessage records the most recent text output.
func (pt *ProgressTracker) SetLastMessage(msg string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if len(msg) > 200 {
		msg = msg[:200] + "…"
	}
	pt.state.LastMessage = msg
	pt.state.UpdatedAt = time.Now()
}

// State returns a snapshot of the current progress.
func (pt *ProgressTracker) State() ProgressState {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.state
}

// FormatProgress returns a human-readable progress string.
func (pt *ProgressTracker) FormatProgress() string {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	s := pt.state
	progress := fmt.Sprintf("Turn %d", s.TurnCount)
	if s.MaxTurns > 0 {
		progress = fmt.Sprintf("Turn %d/%d", s.TurnCount, s.MaxTurns)
	}
	if s.LastTool != "" {
		progress += fmt.Sprintf(" (last tool: %s)", s.LastTool)
	}
	return progress
}

// ProgressRegistry manages progress trackers for multiple agents.
type ProgressRegistry struct {
	mu       sync.RWMutex
	trackers map[string]*ProgressTracker
}

// NewProgressRegistry creates a new registry.
func NewProgressRegistry() *ProgressRegistry {
	return &ProgressRegistry{
		trackers: make(map[string]*ProgressTracker),
	}
}

// Register creates and registers a progress tracker for an agent.
func (pr *ProgressRegistry) Register(agentID string) *ProgressTracker {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pt := NewProgressTracker(agentID)
	pr.trackers[agentID] = pt
	return pt
}

// Get returns the progress tracker for an agent.
func (pr *ProgressRegistry) Get(agentID string) *ProgressTracker {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.trackers[agentID]
}

// Remove removes a progress tracker.
func (pr *ProgressRegistry) Remove(agentID string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	delete(pr.trackers, agentID)
}

// AllProgress returns a snapshot of all active agent progress states.
func (pr *ProgressRegistry) AllProgress() []ProgressState {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	states := make([]ProgressState, 0, len(pr.trackers))
	for _, pt := range pr.trackers {
		states = append(states, pt.State())
	}
	return states
}

// FormatAllProgress returns a multi-line summary of all active agents' progress.
func (pr *ProgressRegistry) FormatAllProgress() string {
	states := pr.AllProgress()
	if len(states) == 0 {
		return "No active agents"
	}

	var result string
	for _, s := range states {
		line := fmt.Sprintf("  %s: Turn %d", truncID(s.AgentID), s.TurnCount)
		if s.MaxTurns > 0 {
			line = fmt.Sprintf("  %s: Turn %d/%d", truncID(s.AgentID), s.TurnCount, s.MaxTurns)
		}
		if s.LastTool != "" {
			line += fmt.Sprintf(" [%s]", s.LastTool)
		}
		result += line + "\n"
	}
	return result
}
