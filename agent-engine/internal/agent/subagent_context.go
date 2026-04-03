package agent

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// SubagentContext — context propagation for spawned subagents.
// Carries parent context, permission constraints, and communication channels.
// Aligned with claude-code-main's subagent context patterns.
// ────────────────────────────────────────────────────────────────────────────

// SubagentContext carries inherited context from a parent agent to a child.
type SubagentContext struct {
	// ParentAgentID is the ID of the agent that spawned this subagent.
	ParentAgentID string `json:"parent_agent_id"`
	// ParentSessionID is the session that owns this subagent tree.
	ParentSessionID string `json:"parent_session_id"`
	// Depth is the nesting level (root = 0, first subagent = 1, etc.).
	Depth int `json:"depth"`
	// MaxDepth is the maximum allowed nesting depth.
	MaxDepth int `json:"max_depth"`
	// TeamName is the swarm/team identifier (if part of a team).
	TeamName string `json:"team_name,omitempty"`

	// ── Permission inheritance ──────────────────────────────────────────
	// InheritedPermittedDirs are directories the parent allows writes to.
	InheritedPermittedDirs []string `json:"inherited_permitted_dirs,omitempty"`
	// InheritedDeniedCommands are commands denied by the parent.
	InheritedDeniedCommands []string `json:"inherited_denied_commands,omitempty"`
	// PermissionMode is the effective permission mode from the parent.
	PermissionMode string `json:"permission_mode,omitempty"`
	// RestrictTools limits which tools the subagent can use.
	RestrictTools []string `json:"restrict_tools,omitempty"`

	// ── Communication ──────────────────────────────────────────────────
	// Mailbox is the subagent's mailbox for receiving messages.
	Mailbox *Mailbox `json:"-"`
	// MailboxRegistry provides access to all agent mailboxes.
	MailboxRegistry *MailboxRegistry `json:"-"`
	// MessageBus provides inter-agent messaging.
	MessageBus *MessageBus `json:"-"`

	// ── Cancellation ───────────────────────────────────────────────────
	// ParentCtx is the parent's context for cascading cancellation.
	ParentCtx context.Context `json:"-"`

	// ── Task metadata ──────────────────────────────────────────────────
	// TaskDescription is the original task assigned to this subagent.
	TaskDescription string `json:"task_description,omitempty"`
	// CreatedAt is when this subagent context was created.
	CreatedAt time.Time `json:"created_at"`
}

// NewSubagentContext creates a subagent context derived from a parent.
func NewSubagentContext(parentID, parentSessionID string, depth int) *SubagentContext {
	return &SubagentContext{
		ParentAgentID:   parentID,
		ParentSessionID: parentSessionID,
		Depth:           depth,
		MaxDepth:        5, // default max nesting
		CreatedAt:       time.Now(),
	}
}

// CanSpawnChild returns true if this subagent is allowed to spawn further children.
func (sc *SubagentContext) CanSpawnChild() bool {
	return sc.Depth < sc.MaxDepth
}

// DeriveChild creates a child SubagentContext inheriting from this one.
func (sc *SubagentContext) DeriveChild(childAgentID string) (*SubagentContext, error) {
	if !sc.CanSpawnChild() {
		return nil, fmt.Errorf("maximum subagent depth (%d) reached", sc.MaxDepth)
	}
	child := &SubagentContext{
		ParentAgentID:           childAgentID,
		ParentSessionID:         sc.ParentSessionID,
		Depth:                   sc.Depth + 1,
		MaxDepth:                sc.MaxDepth,
		TeamName:                sc.TeamName,
		InheritedPermittedDirs:  sc.InheritedPermittedDirs,
		InheritedDeniedCommands: sc.InheritedDeniedCommands,
		PermissionMode:          sc.PermissionMode,
		RestrictTools:           sc.RestrictTools,
		MailboxRegistry:         sc.MailboxRegistry,
		MessageBus:              sc.MessageBus,
		ParentCtx:               sc.ParentCtx,
		CreatedAt:               time.Now(),
	}
	// Create mailbox for the child if registry is available.
	if sc.MailboxRegistry != nil {
		child.Mailbox = sc.MailboxRegistry.GetOrCreate(childAgentID)
	}
	return child, nil
}

// IsToolAllowed checks if a tool is permitted for this subagent.
func (sc *SubagentContext) IsToolAllowed(toolName string) bool {
	if len(sc.RestrictTools) == 0 {
		return true // no restrictions
	}
	for _, t := range sc.RestrictTools {
		if strings.EqualFold(t, toolName) {
			return true
		}
	}
	return false
}

// SendMessage sends a message to another agent via the message bus.
func (sc *SubagentContext) SendMessage(toAgentID string, content interface{}) error {
	if sc.MessageBus == nil {
		return fmt.Errorf("no message bus available")
	}
	return sc.MessageBus.Send(AgentMessage{
		FromAgentID: sc.ParentAgentID,
		ToAgentID:   toAgentID,
		Content:     content,
	})
}

// CheckInbox reads pending messages from the subagent's mailbox.
func (sc *SubagentContext) CheckInbox() []MailboxMessage {
	if sc.Mailbox == nil {
		return nil
	}
	return sc.Mailbox.Read()
}

// ────────────────────────────────────────────────────────────────────────────
// SubagentSummary — generates compact summaries of subagent runs.
// ────────────────────────────────────────────────────────────────────────────

// SubagentSummary holds a compact summary of a completed subagent run.
type SubagentSummary struct {
	AgentID     string        `json:"agent_id"`
	ParentID    string        `json:"parent_id"`
	Task        string        `json:"task"`
	Status      AgentStatus   `json:"status"`
	Duration    time.Duration `json:"duration"`
	TurnCount   int           `json:"turn_count"`
	OutputChars int           `json:"output_chars"`
	Summary     string        `json:"summary"`
	Error       string        `json:"error,omitempty"`
}

// GenerateSubagentSummary creates a compact summary from an AgentTask.
func GenerateSubagentSummary(task *AgentTask) *SubagentSummary {
	summary := &SubagentSummary{
		AgentID:     task.Definition.AgentID,
		ParentID:    task.Definition.ParentID,
		Task:        task.Definition.Task,
		Status:      task.Status,
		OutputChars: len(task.Output),
		Error:       task.Error,
	}

	if !task.StartedAt.IsZero() && !task.FinishedAt.IsZero() {
		summary.Duration = task.FinishedAt.Sub(task.StartedAt)
	}

	// Generate a compact text summary.
	summary.Summary = buildSubagentSummaryText(task)
	return summary
}

// buildSubagentSummaryText creates a human-readable summary of the agent run.
func buildSubagentSummaryText(task *AgentTask) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Agent %s: ", task.Definition.AgentID[:min(8, len(task.Definition.AgentID))]))

	switch task.Status {
	case AgentStatusDone:
		sb.WriteString("completed successfully")
	case AgentStatusFailed:
		sb.WriteString("failed")
		if task.Error != "" {
			errText := task.Error
			if len(errText) > 100 {
				errText = errText[:100] + "..."
			}
			sb.WriteString(": " + errText)
		}
	case AgentStatusCancelled:
		sb.WriteString("was cancelled")
	default:
		sb.WriteString("status: " + string(task.Status))
	}

	if len(task.Output) > 0 {
		sb.WriteString(fmt.Sprintf(" (%s output)", formatOutputSize(len(task.Output))))
	}

	if !task.StartedAt.IsZero() && !task.FinishedAt.IsZero() {
		dur := task.FinishedAt.Sub(task.StartedAt)
		sb.WriteString(fmt.Sprintf(" in %s", dur.Round(time.Second)))
	}

	return sb.String()
}

// TruncateSubagentOutput truncates a subagent's output to fit within the
// parent's context budget.
func TruncateSubagentOutput(output string, maxChars int) string {
	if maxChars <= 0 || len(output) <= maxChars {
		return output
	}
	// Keep the first and last portions.
	headSize := maxChars * 3 / 4
	tailSize := maxChars - headSize - 50 // 50 chars for the truncation marker
	if tailSize < 0 {
		tailSize = 0
	}

	head := output[:headSize]
	tail := ""
	if tailSize > 0 && len(output) > tailSize {
		tail = output[len(output)-tailSize:]
	}

	omitted := len(output) - headSize - tailSize
	return fmt.Sprintf("%s\n\n[... %d characters omitted ...]\n\n%s", head, omitted, tail)
}

func formatOutputSize(chars int) string {
	switch {
	case chars < 1024:
		return fmt.Sprintf("%d chars", chars)
	case chars < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(chars)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(chars)/(1024*1024))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
