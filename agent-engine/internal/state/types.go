package state

import (
	"sync"
	"time"
)

// AppState is the global mutable application state.
// Aligned with claude-code-main's AppState (state/AppStateStore.ts:89-400+).
type AppState struct {
	mu sync.RWMutex

	// ── Core session info ─────────────────────────────────────────────
	CWD          string
	SessionID    string
	TotalCostUSD float64

	// ── Model & mode ─────────────────────────────────────────────────
	CurrentModel     string
	MainLoopModel    string
	Verbose          bool
	AutoMode         bool
	PermissionMode   string // "default", "plan", "bypassPermissions", "auto", "acceptEdits", "dontAsk"
	PlanModeActive   bool
	IsBriefOnly      bool   // concise output mode
	AgentName        string // --agent CLI flag name
	OutputStyle      string // "concise", "verbose", "code-only"
	EffortValue      string // "low", "medium", "high"
	IsNonInteractive bool

	// ── Token / streaming ─────────────────────────────────────────────
	TokenBudgetFraction float64
	InputTokens         int
	OutputTokens        int
	ActiveToolName      string
	IsStreaming         bool
	StatusLineText      string // ephemeral status text for TUI

	// ── Permission context ────────────────────────────────────────────
	ToolPermissionContext *ToolPermissionContextState

	// ── MCP ───────────────────────────────────────────────────────────
	MCP MCPState

	// ── Plugins ───────────────────────────────────────────────────────
	Plugins PluginsState

	// ── Tasks (agent/swarm) ───────────────────────────────────────────
	Tasks              map[string]*TaskState
	ForegroundedTaskID string
	ViewingAgentTaskID string

	// ── Agent name registry ───────────────────────────────────────────
	AgentNameRegistry map[string]string // name → agentID

	// ── File history & attribution ────────────────────────────────────
	FileHistory FileHistoryStateRef
	Attribution AttributionStateRef

	// ── Notifications ─────────────────────────────────────────────────
	Notifications NotificationsState

	// ── Session hooks state ───────────────────────────────────────────
	SessionHooks SessionHooksState

	// ── Thinking / prompt suggestion ──────────────────────────────────
	ThinkingEnabled         *bool // nil = default
	PromptSuggestionEnabled bool
	PromptSuggestion        PromptSuggestionState

	// ── Team / swarm context ──────────────────────────────────────────
	TeamContext *TeamContext

	// ── Inbox (swarm messages) ────────────────────────────────────────
	Inbox InboxState

	// ── Denial tracking ──────────────────────────────────────────────
	DenialTracking DenialTrackingStateRef

	// Listeners notified on any state mutation.
	listeners []func()
}

// ─── Nested state types ───────────────────────────────────────────────────────

// ToolPermissionContextState mirrors the permission context in AppState.
type ToolPermissionContextState struct {
	Mode              string                   `json:"mode"`
	IsBypassAvailable bool                     `json:"is_bypass_available"`
	RulesBySource     map[string][]PermRuleRef `json:"rules_by_source,omitempty"`
	SessionRules      []PermRuleRef            `json:"session_rules,omitempty"`
}

// PermRuleRef is a lightweight permission rule reference for AppState.
type PermRuleRef struct {
	Type     string `json:"type"` // "allow", "deny"
	Pattern  string `json:"pattern"`
	ToolName string `json:"tool_name,omitempty"`
}

// MCPState tracks connected MCP servers and their tools/resources.
type MCPState struct {
	Clients   []MCPClientRef           `json:"clients,omitempty"`
	Tools     []MCPToolRef             `json:"tools,omitempty"`
	Resources map[string][]MCPResource `json:"resources,omitempty"`
}

// MCPClientRef is a lightweight reference to a connected MCP server.
type MCPClientRef struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "connected", "disconnected", "error"
}

// MCPToolRef is a tool provided by an MCP server.
type MCPToolRef struct {
	Name       string `json:"name"`
	ServerName string `json:"server_name"`
}

// MCPResource is a resource provided by an MCP server.
type MCPResource struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// PluginsState tracks loaded plugins.
type PluginsState struct {
	Enabled      []PluginRef   `json:"enabled,omitempty"`
	Disabled     []PluginRef   `json:"disabled,omitempty"`
	Errors       []PluginError `json:"errors,omitempty"`
	NeedsRefresh bool          `json:"needs_refresh,omitempty"`
}

// PluginRef is a lightweight reference to a loaded plugin.
type PluginRef struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Path    string `json:"path,omitempty"`
}

// PluginError records a plugin loading/initialization error.
type PluginError struct {
	PluginName string `json:"plugin_name"`
	Error      string `json:"error"`
	Phase      string `json:"phase,omitempty"` // "load", "init", "runtime"
}

// TaskState tracks the state of a single agent task.
type TaskState struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Status      string `json:"status"` // "pending", "running", "completed", "failed", "cancelled"
	Description string `json:"description,omitempty"`
	AgentID     string `json:"agent_id,omitempty"`
	AgentType   string `json:"agent_type,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	CompletedAt int64  `json:"completed_at,omitempty"`
}

// FileHistoryStateRef is the AppState view of file history.
type FileHistoryStateRef struct {
	Files map[string][]FileSnapshotRef `json:"files,omitempty"`
}

// FileSnapshotRef is a snapshot record in AppState.
type FileSnapshotRef struct {
	Timestamp int64  `json:"timestamp"`
	Hash      string `json:"hash"`
	ToolUseID string `json:"tool_use_id,omitempty"`
}

// AttributionStateRef is the AppState view of commit attribution.
type AttributionStateRef struct {
	DirtyFiles     map[string]string `json:"dirty_files,omitempty"`
	CommittedFiles map[string]string `json:"committed_files,omitempty"`
}

// NotificationsState tracks pending notifications.
type NotificationsState struct {
	Current *NotificationRef  `json:"current,omitempty"`
	Queue   []NotificationRef `json:"queue,omitempty"`
}

// NotificationRef is a lightweight notification reference.
type NotificationRef struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Type    string `json:"type,omitempty"` // "info", "warning", "error"
}

// SessionHooksState tracks session-level hook state.
type SessionHooksState struct {
	ActiveHooks    []string          `json:"active_hooks,omitempty"`
	LastRunResults map[string]string `json:"last_run_results,omitempty"`
}

// PromptSuggestionState tracks prompt suggestion state.
type PromptSuggestionState struct {
	Text                string `json:"text,omitempty"`
	PromptID            string `json:"prompt_id,omitempty"` // "user_intent", "stated_intent"
	ShownAt             int64  `json:"shown_at,omitempty"`
	AcceptedAt          int64  `json:"accepted_at,omitempty"`
	GenerationRequestID string `json:"generation_request_id,omitempty"`
}

// TeamContext tracks team/swarm context for multi-agent sessions.
type TeamContext struct {
	TeamName      string                 `json:"team_name"`
	TeamFilePath  string                 `json:"team_file_path,omitempty"`
	LeadAgentID   string                 `json:"lead_agent_id"`
	SelfAgentID   string                 `json:"self_agent_id,omitempty"`
	SelfAgentName string                 `json:"self_agent_name,omitempty"`
	IsLeader      bool                   `json:"is_leader,omitempty"`
	Teammates     map[string]TeammateRef `json:"teammates,omitempty"`
}

// TeammateRef tracks a single teammate in a swarm.
type TeammateRef struct {
	Name      string `json:"name"`
	AgentType string `json:"agent_type,omitempty"`
	CWD       string `json:"cwd,omitempty"`
	SpawnedAt int64  `json:"spawned_at"`
}

// InboxState tracks incoming messages for swarm members.
type InboxState struct {
	Messages []InboxMessage `json:"messages,omitempty"`
}

// InboxMessage is a single message in the swarm inbox.
type InboxMessage struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"` // "pending", "processing", "processed"
}

// DenialTrackingStateRef is the AppState view of denial tracking.
type DenialTrackingStateRef struct {
	ConsecutiveDenials int  `json:"consecutive_denials"`
	FailClosed         bool `json:"fail_closed"`
}

// New creates a fresh AppState with sane defaults.
func New(cwd string) *AppState {
	return &AppState{
		CWD:               cwd,
		PermissionMode:    "default",
		Tasks:             make(map[string]*TaskState),
		AgentNameRegistry: make(map[string]string),
	}
}

// Get returns a snapshot copy (safe for reading outside a lock).
// Note: nested maps/slices are shallow-copied.
func (s *AppState) Get() AppState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copy := *s
	copy.listeners = nil // don't leak listeners
	return copy
}

// Update applies fn under a write lock and notifies all listeners.
func (s *AppState) Update(fn func(st *AppState)) {
	s.mu.Lock()
	fn(s)
	listeners := make([]func(), len(s.listeners))
	copy(listeners, s.listeners)
	s.mu.Unlock()

	for _, l := range listeners {
		l()
	}
}

// Subscribe registers a callback invoked after every Update.
func (s *AppState) Subscribe(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, fn)
}
