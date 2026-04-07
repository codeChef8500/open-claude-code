package engine

import (
	"context"
	"encoding/json"
	"sync"
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
	IsReadOnly(input json.RawMessage) bool
	IsConcurrencySafe(input json.RawMessage) bool
	// IsDestructive reports whether the tool makes irreversible changes.
	IsDestructive(input json.RawMessage) bool
	// IsTransparentWrapper reports whether the tool's underlying operations
	// should be reported to the user rather than the wrapper name.
	IsTransparentWrapper() bool
	// IsSearchOrRead reports whether the tool is a search/read operation
	// (used for UI collapsing and activity classification).
	IsSearchOrRead(input json.RawMessage) SearchOrReadInfo
	// AlwaysLoad reports whether the tool definition must always be included
	// in the system prompt regardless of session settings.
	AlwaysLoad() bool
	// ShouldDefer reports whether the tool should be deferred in plan mode.
	ShouldDefer() bool
	// IsMCP reports whether this tool was provided by an MCP server.
	IsMCP() bool
	// IsLSP reports whether this tool was provided by an LSP server.
	IsLSP() bool
	// IsOpenWorld reports whether the tool interacts with external services.
	IsOpenWorld(input json.RawMessage) bool
	// RequiresUserInteraction reports whether the tool needs interactive user input.
	RequiresUserInteraction() bool
	// Strict reports whether strict mode is enabled for this tool.
	Strict() bool

	// ── Configuration ────────────────────────────────────────────────────
	MaxResultSizeChars() int
	// InterruptBehavior returns the interrupt-handling policy for this tool.
	InterruptBehavior() InterruptBehavior
	// Aliases returns alternate names that route to this tool.
	Aliases() []string
	// OutputSchema returns the JSON Schema for the tool's output, or nil.
	OutputSchema() json.RawMessage

	// ── Classification & permission helpers ──────────────────────────────
	// ToAutoClassifierInput returns a compact representation for the auto-mode
	// security classifier. Return "" to skip this tool in the classifier.
	ToAutoClassifierInput(input json.RawMessage) string
	// InputsEquivalent reports whether two inputs are functionally equivalent.
	InputsEquivalent(a, b json.RawMessage) bool
	// PreparePermissionMatcher returns a matcher function for hook "if" conditions.
	// Returns nil if not implemented (only tool-name matching works).
	PreparePermissionMatcher(input json.RawMessage) func(pattern string) bool
	// BackfillObservableInput mutates input copies before observers see them.
	BackfillObservableInput(input map[string]interface{})
	// MapToolResultToBlockParam converts tool output to the API tool_result format.
	MapToolResultToBlockParam(content interface{}, toolUseID string) *ContentBlock
	// IsResultTruncated reports whether the non-verbose rendering of this
	// output is truncated (i.e. expanding would reveal more content).
	IsResultTruncated(output interface{}) bool

	// ── Context modifier ────────────────────────────────────────────────
	// ContextModifier returns a function that modifies the UseContext after this
	// tool runs (only for non-concurrency-safe tools). Nil = no modification.
	ContextModifier() func(uctx *UseContext) *UseContext

	// ── MCP info ────────────────────────────────────────────────────────
	// MCPInfo returns the MCP server/tool info if this is an MCP tool, or nil.
	MCPInfo() *MCPToolInfo

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

// SearchOrReadInfo describes whether a tool use is a search, read, or list operation.
type SearchOrReadInfo struct {
	IsSearch bool
	IsRead   bool
	IsList   bool
}

// MCPToolInfo carries the MCP server/tool identity for MCP-provided tools.
type MCPToolInfo struct {
	ServerName string `json:"server_name"`
	ToolName   string `json:"tool_name"`
}

// UseContext carries per-request context that tools may need.
// Aligned with claude-code-main's ToolUseContext (Tool.ts:158-300).
type UseContext struct {
	// ── Core identity ────────────────────────────────────────────────────
	WorkDir    string
	SessionID  string
	AgentID    string
	AgentType  string // subagent type name
	TeammateID string // sub-agent context in swarm sessions

	// ── Permission & mode ───────────────────────────────────────────────
	AutoMode                bool
	PlanModeActive          bool
	PermissionMode          string // "default", "auto", "bypass", "plan", "acceptEdits", "dontAsk"
	PermittedDirs           []string
	DeniedCommands          []string
	IsNonInteractiveSession bool

	// ── Model info ──────────────────────────────────────────────────────
	MainLoopModel string
	Verbose       bool
	Debug         bool

	// ── Callbacks ───────────────────────────────────────────────────────
	AskPermission       func(ctx context.Context, tool, desc string) (bool, error)
	SendNotification    func(msg string)
	AppendSystemMessage func(msg *Message)
	SetToolJSX          func(toolUseID string, jsx interface{})

	// ── Progress reporting ──────────────────────────────────────────
	// OnToolProgress, if non-nil, is called by tools to emit incremental
	// progress events (e.g. bash output lines, web search progress).
	OnToolProgress func(progress *ProgressData)

	// ── Abort / cancellation ────────────────────────────────────────
	AbortCh <-chan struct{} // closed when the query is cancelled

	// ── File state cache (LRU for file contents read by tools) ─────────
	ReadFileState *FileStateCache

	// ── Content replacement state (tool result budget) ─────────────────
	ContentReplacementState *ContentReplacementState

	// ── Query tracking ─────────────────────────────────────────────────
	QueryTracking *QueryChainTracking

	// ── Tool decisions (accept/reject per tool use ID) ─────────────────
	ToolDecisions *ToolDecisionMap

	// ── File reading limits ────────────────────────────────────────────
	FileReadingLimits *FileReadingLimits

	// ── Glob limits ────────────────────────────────────────────────────
	GlobLimits *GlobLimits

	// ── Memory / CLAUDE.md triggers ────────────────────────────────────
	NestedMemoryAttachmentTriggers map[string]struct{}
	LoadedNestedMemoryPaths        map[string]struct{}

	// ── Skill discovery ────────────────────────────────────────────────
	DynamicSkillDirTriggers map[string]struct{}
	DiscoveredSkillNames    map[string]struct{}

	// ── In-progress tool tracking ───────────────────────────────────────
	SetInProgressToolUseIDs func(update func(prev map[string]struct{}) map[string]struct{})
	SetResponseLength       func(update func(prev int) int)

	// ── File history & attribution updaters ─────────────────────────────
	UpdateFileHistoryState func(updater func(prev *FileHistoryState) *FileHistoryState)
	UpdateAttributionState func(updater func(prev *AttributionState) *AttributionState)

	// ── AppState access ────────────────────────────────────────────────
	GetAppState func() interface{}
	SetAppState func(f func(prev interface{}) interface{})

	// ── Current messages (for tools that need conversation context) ────
	Messages []*Message

	// ── Per-call overrides ─────────────────────────────────────────────
	MaxResultChars int    // overrides tool's MaxResultSizeChars
	ToolUseID      string // current tool_use block ID

	// ── Prompt elicitation (interactive hooks) ─────────────────────────
	RequestPrompt func(sourceName string, toolInputSummary string) func(request interface{}) (interface{}, error)

	// ── Plan mode state ────────────────────────────────────────────────
	SetPlanState func(title, plan string, approved bool)

	// ── Task management ────────────────────────────────────────────────
	TaskRegistry TaskRegistry

	// ── Denial tracking (for async subagents) ──────────────────────────
	LocalDenialTracking *DenialTrackingState

	// ── Experimental ───────────────────────────────────────────────────
	CriticalSystemReminder string
	PreserveToolUseResults bool
	RequireCanUseTool      bool
	UserModified           bool
}

// ── Supporting types referenced by UseContext ────────────────────────────────

// FileReadingLimits constrains how much data a file-reading tool may return.
type FileReadingLimits struct {
	MaxTokens    int `json:"max_tokens,omitempty"`
	MaxSizeBytes int `json:"max_size_bytes,omitempty"`
}

// GlobLimits constrains glob result counts.
type GlobLimits struct {
	MaxResults int `json:"max_results,omitempty"`
}

// ToolDecision records a per-tool-use accept/reject decision.
type ToolDecision struct {
	Source    string `json:"source"`
	Decision  string `json:"decision"` // "accept" or "reject"
	Timestamp int64  `json:"timestamp"`
}

// ToolDecisionMap is a thread-safe map of tool-use-ID → ToolDecision.
type ToolDecisionMap struct {
	mu sync.RWMutex
	m  map[string]ToolDecision
}

// NewToolDecisionMap creates an empty decision map.
func NewToolDecisionMap() *ToolDecisionMap {
	return &ToolDecisionMap{m: make(map[string]ToolDecision)}
}

// Set records a decision for a tool use.
func (d *ToolDecisionMap) Set(toolUseID string, dec ToolDecision) {
	d.mu.Lock()
	d.m[toolUseID] = dec
	d.mu.Unlock()
}

// Get returns the decision for a tool use, if any.
func (d *ToolDecisionMap) Get(toolUseID string) (ToolDecision, bool) {
	d.mu.RLock()
	dec, ok := d.m[toolUseID]
	d.mu.RUnlock()
	return dec, ok
}

// QueryChainTracking tracks the chain of queries for a conversation turn.
type QueryChainTracking struct {
	TurnIndex         int   `json:"turn_index"`
	ChainLength       int   `json:"chain_length"`
	TotalToolCalls    int   `json:"total_tool_calls"`
	TotalInputTokens  int   `json:"total_input_tokens"`
	TotalOutputTokens int   `json:"total_output_tokens"`
	StartTime         int64 `json:"start_time"` // unix millis
}

// DenialTrackingState tracks consecutive denials for fail-closed triggering.
type DenialTrackingState struct {
	ConsecutiveDenials int  `json:"consecutive_denials"`
	FailClosed         bool `json:"fail_closed"`
}

// FileHistoryState tracks file modification history during a session.
type FileHistoryState struct {
	// Files maps absolute path → list of snapshots.
	Files map[string][]FileSnapshot `json:"files,omitempty"`
}

// FileSnapshot is a point-in-time record of a file's state.
type FileSnapshot struct {
	Timestamp int64  `json:"timestamp"`
	Hash      string `json:"hash"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`
}

// AttributionState tracks commit attribution data during a session.
type AttributionState struct {
	// DirtyFiles maps absolute path → tool that last modified it.
	DirtyFiles map[string]string `json:"dirty_files,omitempty"`
	// CommittedFiles maps absolute path → commit SHA.
	CommittedFiles map[string]string `json:"committed_files,omitempty"`
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
