package engine

import (
	"context"
	"encoding/json"
)

// InterruptBehavior controls what happens to a running tool when the query
// loop is cancelled or interrupted by the user.
type InterruptBehavior string

const (
	InterruptBehaviorNone   InterruptBehavior = "none"   // allow the tool to complete
	InterruptBehaviorStop   InterruptBehavior = "stop"   // stop immediately, discard output
	InterruptBehaviorReturn InterruptBehavior = "return" // stop and return any partial output
)

// ToolProgressData carries incremental progress emitted by a long-running tool call.
type ToolProgressData struct {
	ToolUseID string `json:"tool_use_id"`
	Text      string `json:"text"`
}

// Tool is the contract every tool implementation must satisfy.
// Defining it here (rather than in the tool package) breaks the
// engine ↔ tool import cycle.
//
// The 13 new methods (ValidateInput … OutputSchema) all have sensible defaults
// in tool.BaseTool; embed that struct to avoid boilerplate.
type Tool interface {
	// ── Core identity ────────────────────────────────────────────────────
	Name() string
	UserFacingName() string
	Description() string
	InputSchema() json.RawMessage

	// ── Execution ────────────────────────────────────────────────────────
	Call(ctx context.Context, input json.RawMessage, uctx *UseContext) (<-chan *ContentBlock, error)
	CheckPermissions(ctx context.Context, input json.RawMessage, uctx *UseContext) error
	// ValidateInput checks structural validity before permission evaluation.
	ValidateInput(ctx context.Context, input json.RawMessage) error

	// ── Prompt integration ───────────────────────────────────────────────
	Prompt(uctx *UseContext) string

	// ── Feature flags ────────────────────────────────────────────────────
	IsEnabled(uctx *UseContext) bool
	IsReadOnly() bool
	IsConcurrencySafe() bool
	// IsDestructive reports whether the tool makes irreversible changes.
	IsDestructive() bool
	// IsTransparentWrapper reports whether the tool's underlying operations
	// should be reported to the user rather than the wrapper name.
	IsTransparentWrapper() bool
	// IsSearchOrRead reports whether the tool is a search/read operation
	// (used for UI collapsing and activity classification).
	IsSearchOrRead() bool
	// AlwaysLoad reports whether the tool definition must always be included
	// in the system prompt regardless of session settings.
	AlwaysLoad() bool
	// ShouldDefer reports whether the tool should be deferred in plan mode.
	ShouldDefer() bool

	// ── Configuration ────────────────────────────────────────────────────
	MaxResultSizeChars() int
	// InterruptBehavior returns the interrupt-handling policy for this tool.
	InterruptBehavior() InterruptBehavior
	// Aliases returns alternate names that route to this tool.
	Aliases() []string
	// OutputSchema returns the JSON Schema for the tool's output, or nil.
	OutputSchema() json.RawMessage

	// ── UI helpers ───────────────────────────────────────────────────────
	// GetPath extracts a filesystem path from the tool input, or "".
	GetPath(input json.RawMessage) string
	// SearchHint returns a short descriptive hint for search classification.
	SearchHint() string
	// GetActivityDescription returns a human-readable description of what
	// the tool is doing, given its current input (used in progress UI).
	GetActivityDescription(input json.RawMessage) string
	// GetToolUseSummary returns a compact summary of the tool use for display.
	GetToolUseSummary(input json.RawMessage) string
}

// UseContext carries per-request context that tools may need.
type UseContext struct {
	WorkDir          string
	SessionID        string
	AutoMode         bool
	AgentID          string
	PermittedDirs    []string
	DeniedCommands   []string
	AskPermission    func(ctx context.Context, tool, desc string) (bool, error)
	SendNotification func(msg string)

	// PlanModeActive is true when the session is in plan-only mode.
	PlanModeActive bool
	// TeammateID identifies the sub-agent context in swarm sessions.
	TeammateID string
	// MaxResultChars overrides the tool's MaxResultSizeChars for this call.
	MaxResultChars int
	// PermissionMode is the current permission enforcement mode
	// ("normal", "auto", "bypass").
	PermissionMode string
	// TaskRegistry is an optional interface for task lifecycle management.
	// Nil when task management is not wired into this session.
	TaskRegistry TaskRegistry
}

// TaskRegistry is the minimal interface that task-management tools use to
// create, update, and query tasks.  The concrete implementation lives in the
// agent package; the interface is defined here to avoid import cycles.
type TaskRegistry interface {
	Create(id, title, description, priority string)
	Update(id string, fields map[string]interface{}) error
	Get(id string) (map[string]interface{}, bool)
	List() []map[string]interface{}
}
