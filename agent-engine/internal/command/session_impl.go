package command

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// Session management command deep implementations.
// Aligned with claude-code-main commands/resume, session, branch, diff, rewind.
// ──────────────────────────────────────────────────────────────────────────────

// ─── /resume deep implementation ────────────────────────────────────────────
// Aligned with claude-code-main commands/resume/resume.tsx.

// ResumeViewData is the structured data for the resume TUI component.
type ResumeViewData struct {
	// Sessions is the list of past sessions to choose from.
	Sessions []ResumeSessionEntry `json:"sessions,omitempty"`
	// SelectedID is the pre-selected session ID (from args).
	SelectedID string `json:"selected_id,omitempty"`
	// SearchQuery is a title/ID search filter.
	SearchQuery string `json:"search_query,omitempty"`
	// Error message if session listing failed.
	Error string `json:"error,omitempty"`
}

// ResumeSessionEntry is a display-friendly session for the resume picker.
type ResumeSessionEntry struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	CreatedAt    string `json:"created_at"` // human-readable
	UpdatedAt    string `json:"updated_at"` // human-readable
	MessageCount int    `json:"message_count"`
	Model        string `json:"model"`
	CWD          string `json:"cwd"`
}

// DeepResumeCommand replaces the basic ResumeCommand with full logic.
type DeepResumeCommand struct{ BaseCommand }

func (c *DeepResumeCommand) Name() string                  { return "resume" }
func (c *DeepResumeCommand) Aliases() []string             { return []string{"continue"} }
func (c *DeepResumeCommand) Description() string           { return "Resume a previous conversation" }
func (c *DeepResumeCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepResumeCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *DeepResumeCommand) ArgumentHint() string          { return "[session-id or search]" }

func (c *DeepResumeCommand) ExecuteInteractive(ctx context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &ResumeViewData{}

	if len(args) > 0 {
		// Could be a session ID or search query.
		data.SelectedID = args[0]
		data.SearchQuery = strings.Join(args, " ")
	}

	// Fetch session list from SessionService.
	if ectx != nil && ectx.Services != nil && ectx.Services.Session != nil {
		sessions, err := ectx.Services.Session.ListSessions(ctx)
		if err != nil {
			data.Error = fmt.Sprintf("Failed to list sessions: %v", err)
		} else {
			for _, s := range sessions {
				entry := ResumeSessionEntry{
					ID:           s.ID,
					Title:        s.Title,
					CreatedAt:    formatTimestamp(s.CreatedAt),
					UpdatedAt:    formatTimestamp(s.UpdatedAt),
					MessageCount: s.MessageCount,
					Model:        s.Model,
					CWD:          s.CWD,
				}
				// Apply search filter if present.
				if data.SearchQuery != "" {
					q := strings.ToLower(data.SearchQuery)
					if !strings.Contains(strings.ToLower(entry.Title), q) &&
						!strings.Contains(strings.ToLower(entry.ID), q) {
						continue
					}
				}
				data.Sessions = append(data.Sessions, entry)
			}
		}
	}

	return &InteractiveResult{
		Component: "resume",
		Data:      data,
	}, nil
}

// ─── /session deep implementation ───────────────────────────────────────────
// Aligned with claude-code-main commands/session/session.tsx.

// SessionViewData is the structured data for the session info panel.
type SessionViewData struct {
	SessionID      string `json:"session_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	WorkDir        string `json:"work_dir"`
	Model          string `json:"model"`
	TurnCount      int    `json:"turn_count"`
	TotalTokens    int    `json:"total_tokens"`
	CostUSD        string `json:"cost_usd"`
	PlanMode       bool   `json:"plan_mode"`
	FastMode       bool   `json:"fast_mode"`
	AutoMode       bool   `json:"auto_mode"`
	Effort         string `json:"effort"`
	Permission     string `json:"permission_mode"`
	MCPServers     int    `json:"mcp_servers"`
	// For remote connection
	RemoteURL    string `json:"remote_url,omitempty"`
	FallbackText string `json:"fallback_text,omitempty"`
}

// DeepSessionCommand replaces the basic SessionCommand with full logic.
type DeepSessionCommand struct{ BaseCommand }

func (c *DeepSessionCommand) Name() string                  { return "session" }
func (c *DeepSessionCommand) Aliases() []string             { return []string{"remote"} }
func (c *DeepSessionCommand) Description() string           { return "Show session info and remote URL" }
func (c *DeepSessionCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepSessionCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepSessionCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &SessionViewData{}
	if ectx != nil {
		data.SessionID = ectx.SessionID
		data.WorkDir = ectx.WorkDir
		data.Model = ectx.Model
		data.TurnCount = ectx.TurnCount
		data.TotalTokens = ectx.TotalTokens
		data.CostUSD = fmt.Sprintf("$%.4f", ectx.CostUSD)
		data.PlanMode = ectx.PlanModeActive
		data.FastMode = ectx.FastMode
		data.AutoMode = ectx.AutoMode
		data.Effort = ectx.EffortLevel
		data.Permission = ectx.PermissionMode
		data.MCPServers = len(ectx.ActiveMCPServers)
	}

	data.FallbackText = buildSessionText(data)
	return &InteractiveResult{
		Component: "session",
		Data:      data,
	}, nil
}

func buildSessionText(d *SessionViewData) string {
	lines := []string{
		fmt.Sprintf("Session:  %s", d.SessionID),
		fmt.Sprintf("WorkDir:  %s", d.WorkDir),
		fmt.Sprintf("Model:    %s", d.Model),
		fmt.Sprintf("Turns:    %d", d.TurnCount),
		fmt.Sprintf("Tokens:   %d", d.TotalTokens),
		fmt.Sprintf("Cost:     %s", d.CostUSD),
	}
	if d.Effort != "" {
		lines = append(lines, fmt.Sprintf("Effort:   %s", d.Effort))
	}
	if d.Permission != "" {
		lines = append(lines, fmt.Sprintf("Perms:    %s", d.Permission))
	}
	if d.RemoteURL != "" {
		lines = append(lines, fmt.Sprintf("Remote:   %s", d.RemoteURL))
	}
	return strings.Join(lines, "\n")
}

// ─── /diff deep implementation ──────────────────────────────────────────────
// Aligned with claude-code-main commands/diff/diff.tsx.

// DiffViewData is the structured data for the diff TUI component.
type DiffViewData struct {
	// UncommittedDiff is the raw git diff output.
	UncommittedDiff string `json:"uncommitted_diff,omitempty"`
	// StagedDiff is the staged changes diff.
	StagedDiff string `json:"staged_diff,omitempty"`
	// FilesChanged lists modified files.
	FilesChanged []string `json:"files_changed,omitempty"`
	// TurnDiffs maps turn number to diff content (per-turn change tracking).
	TurnDiffs []TurnDiffEntry `json:"turn_diffs,omitempty"`
	// Error if diff retrieval failed.
	Error string `json:"error,omitempty"`
}

// TurnDiffEntry represents changes made during a specific turn.
type TurnDiffEntry struct {
	Turn    int      `json:"turn"`
	Summary string   `json:"summary"`
	Files   []string `json:"files"`
	Diff    string   `json:"diff,omitempty"`
}

// DeepDiffCommand replaces the basic DiffCommand with full logic.
type DeepDiffCommand struct{ BaseCommand }

func (c *DeepDiffCommand) Name() string                  { return "diff" }
func (c *DeepDiffCommand) Description() string           { return "Show uncommitted changes and per-turn diffs" }
func (c *DeepDiffCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepDiffCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepDiffCommand) ExecuteInteractive(ctx context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &DiffViewData{}

	if ectx != nil && ectx.Services != nil && ectx.Services.Git != nil {
		git := ectx.Services.Git

		// Get uncommitted changes.
		diff, err := git.Diff(ctx, ectx.WorkDir)
		if err != nil {
			data.Error = fmt.Sprintf("git diff failed: %v", err)
		} else {
			data.UncommittedDiff = diff
		}

		// Get staged changes.
		staged, err := git.Diff(ctx, ectx.WorkDir, "--cached")
		if err == nil {
			data.StagedDiff = staged
		}

		// Get changed file list.
		status, err := git.Status(ctx, ectx.WorkDir)
		if err == nil {
			for _, line := range strings.Split(status, "\n") {
				line = strings.TrimSpace(line)
				if line != "" && len(line) > 3 {
					data.FilesChanged = append(data.FilesChanged, strings.TrimSpace(line[2:]))
				}
			}
		}
	}

	return &InteractiveResult{
		Component: "diff",
		Data:      data,
	}, nil
}

// ─── /rewind deep implementation ────────────────────────────────────────────
// Aligned with claude-code-main commands/rewind/rewind.tsx.

// RewindViewData is the structured data for the rewind TUI component.
type RewindViewData struct {
	// Checkpoints are the available rewind points.
	Checkpoints []RewindCheckpoint `json:"checkpoints,omitempty"`
	// SelectedHash is a pre-selected checkpoint (from args).
	SelectedHash string `json:"selected_hash,omitempty"`
	// Error if checkpoint listing failed.
	Error string `json:"error,omitempty"`
}

// RewindCheckpoint represents a restorable point in time.
type RewindCheckpoint struct {
	Hash      string `json:"hash"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Turn      int    `json:"turn"`
	IsCurrent bool   `json:"is_current"`
}

// DeepRewindCommand replaces the basic RewindCommand with full logic.
type DeepRewindCommand struct{ BaseCommand }

func (c *DeepRewindCommand) Name() string      { return "rewind" }
func (c *DeepRewindCommand) Aliases() []string { return []string{"checkpoint"} }
func (c *DeepRewindCommand) Description() string {
	return "Restore code and/or conversation to a previous point"
}
func (c *DeepRewindCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepRewindCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepRewindCommand) ExecuteInteractive(ctx context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &RewindViewData{}

	if len(args) > 0 {
		data.SelectedHash = args[0]
	}

	// Fetch checkpoints from GitService.
	if ectx != nil && ectx.Services != nil && ectx.Services.Git != nil {
		checkpoints, err := ectx.Services.Git.ListCheckpoints(ctx, ectx.WorkDir)
		if err != nil {
			data.Error = fmt.Sprintf("Failed to list checkpoints: %v", err)
		} else {
			for _, cp := range checkpoints {
				data.Checkpoints = append(data.Checkpoints, RewindCheckpoint{
					Hash:      cp.Hash,
					Message:   cp.Message,
					Timestamp: formatTimestamp(cp.Timestamp),
					Turn:      cp.Turn,
				})
			}
			if len(data.Checkpoints) > 0 {
				data.Checkpoints[0].IsCurrent = true
			}
		}
	}

	return &InteractiveResult{
		Component: "rewind",
		Data:      data,
	}, nil
}

// ─── /branch deep implementation ────────────────────────────────────────────
// Aligned with claude-code-main commands/branch/branch.ts.

// BranchViewData is the structured data for the branch TUI component.
type BranchViewData struct {
	BranchName string `json:"branch_name"`
	SessionID  string `json:"session_id"`
	TurnCount  int    `json:"turn_count"`
	Message    string `json:"message,omitempty"`
	Error      string `json:"error,omitempty"`
}

// DeepBranchCommand replaces the basic BranchCommand with full logic.
type DeepBranchCommand struct{ BaseCommand }

func (c *DeepBranchCommand) Name() string { return "branch" }
func (c *DeepBranchCommand) Description() string {
	return "Create a branch of the current conversation at this point"
}
func (c *DeepBranchCommand) ArgumentHint() string          { return "[name]" }
func (c *DeepBranchCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepBranchCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepBranchCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &BranchViewData{}

	if len(args) > 0 {
		data.BranchName = strings.Join(args, " ")
	}

	if ectx != nil {
		data.SessionID = ectx.SessionID
		data.TurnCount = ectx.TurnCount
	}

	return &InteractiveResult{
		Component: "branch",
		Data:      data,
	}, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

// formatTimestamp formats a Unix timestamp to a human-readable string.
func formatTimestamp(ts int64) string {
	if ts <= 0 {
		return ""
	}
	t := time.Unix(ts, 0)
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("Jan 2, 2006 15:04")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Register deep session commands, replacing stubs.
// ──────────────────────────────────────────────────────────────────────────────

func init() {
	defaultRegistry.RegisterOrReplace(
		&DeepResumeCommand{},
		&DeepSessionCommand{},
		&DeepDiffCommand{},
		&DeepRewindCommand{},
		&DeepBranchCommand{},
	)
}
