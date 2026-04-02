package engine

import (
	"encoding/json"
	"time"
)

// StreamEventType enumerates all event types emitted by the engine.
type StreamEventType string

const (
	EventTextDelta       StreamEventType = "text_delta"
	EventTextComplete    StreamEventType = "text_complete"
	EventToolUse         StreamEventType = "tool_use"
	EventToolResult      StreamEventType = "tool_result"
	EventThinking        StreamEventType = "thinking"
	EventUsage           StreamEventType = "usage"
	EventError           StreamEventType = "error"
	EventDone            StreamEventType = "done"
	EventSystemMessage   StreamEventType = "system_message"
	EventToolProgress    StreamEventType = "tool_progress"
	EventRequestStart    StreamEventType = "request_start"
	EventTombstone       StreamEventType = "tombstone"
	EventToolUseSummary  StreamEventType = "tool_use_summary"
	EventProgress        StreamEventType = "progress"
	EventAttachment      StreamEventType = "attachment"
	EventCompactBoundary StreamEventType = "compact_boundary"
)

// StreamEvent is produced by the engine and consumed by SDK callers or HTTP SSE.
type StreamEvent struct {
	Type      StreamEventType `json:"type"`
	Text      string          `json:"text,omitempty"`
	ToolName  string          `json:"tool_name,omitempty"`
	ToolID    string          `json:"tool_id,omitempty"`
	ToolInput interface{}     `json:"tool_input,omitempty"`
	Result    string          `json:"result,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	Thinking  string          `json:"thinking,omitempty"`
	Usage     *UsageStats     `json:"usage,omitempty"`
	Error     string          `json:"error,omitempty"`
	SessionID string          `json:"session_id,omitempty"`

	// Extended fields for richer event types.
	Progress       *ProgressData        `json:"progress,omitempty"`
	Attachment     *AttachmentData      `json:"attachment,omitempty"`
	Tombstone      *TombstoneData       `json:"tombstone,omitempty"`
	ToolUseSummary *ToolUseSummaryData  `json:"tool_use_summary,omitempty"`
	CompactInfo    *CompactBoundaryData `json:"compact_info,omitempty"`
	MessageUUID    string               `json:"message_uuid,omitempty"`
	Level          string               `json:"level,omitempty"` // info, warning, error
}

// UsageStats carries token and cost information from an LLM response.
type UsageStats struct {
	InputTokens              int     `json:"input_tokens"`
	OutputTokens             int     `json:"output_tokens"`
	CacheCreationInputTokens int     `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int     `json:"cache_read_input_tokens,omitempty"`
	CacheDeletedInputTokens  int     `json:"cache_deleted_input_tokens,omitempty"`
	CostUSD                  float64 `json:"cost_usd,omitempty"`
	ServerDurationMs         int     `json:"server_duration_ms,omitempty"`
}

// ProgressData carries incremental progress from a long-running tool or hook.
type ProgressData struct {
	ToolUseID    string `json:"tool_use_id,omitempty"`
	ParentToolID string `json:"parent_tool_use_id,omitempty"`
	Content      string `json:"content,omitempty"`
	SpinnerMode  string `json:"spinner_mode,omitempty"` // "dots", "line", "hidden"
}

// AttachmentData carries metadata for an attachment message (memory, hook output, etc.).
type AttachmentData struct {
	Type        string `json:"type"` // "memory", "hook_output", "hook_error", "hook_blocking_error", "hook_cancelled", "hook_permission_decision", "skill", "file_change"
	Content     string `json:"content,omitempty"`
	ToolUseID   string `json:"tool_use_id,omitempty"`
	HookName    string `json:"hook_name,omitempty"`
	HookEvent   string `json:"hook_event,omitempty"`
	FilePath    string `json:"file_path,omitempty"`
	MemoryTitle string `json:"memory_title,omitempty"`
}

// TombstoneData marks a message that should be removed from UI and transcript.
type TombstoneData struct {
	MessageUUID string `json:"message_uuid"`
	Reason      string `json:"reason,omitempty"`
}

// ToolUseSummaryData carries a compact summary of a completed tool use.
type ToolUseSummaryData struct {
	ToolUseID  string `json:"tool_use_id"`
	ToolName   string `json:"tool_name"`
	Summary    string `json:"summary"`
	DurationMs int    `json:"duration_ms,omitempty"`
}

// CompactBoundaryData carries information about a compaction event.
type CompactBoundaryData struct {
	TokensFreed      int    `json:"tokens_freed,omitempty"`
	TokensRemaining  int    `json:"tokens_remaining,omitempty"`
	Direction        string `json:"direction,omitempty"` // "forward", "backward"
	SummaryMessageID string `json:"summary_message_id,omitempty"`
}

// MessageRole mirrors the Anthropic / OpenAI role conventions.
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
	RoleTool      MessageRole = "tool"
)

// ContentType enumerates the types of content blocks in a message.
type ContentType string

const (
	ContentTypeText       ContentType = "text"
	ContentTypeImage      ContentType = "image"
	ContentTypeToolUse    ContentType = "tool_use"
	ContentTypeToolResult ContentType = "tool_result"
	ContentTypeThinking   ContentType = "thinking"
	ContentTypeDocument   ContentType = "document"
)

// ContentType additional types for extended blocks.
const (
	ContentTypeRedactedThinking ContentType = "redacted_thinking"
	ContentTypeServerToolUse    ContentType = "server_tool_use"
	ContentTypeServerToolResult ContentType = "server_tool_result"
	ContentTypeMCPToolUse       ContentType = "mcp_tool_use"
	ContentTypeMCPToolResult    ContentType = "mcp_tool_result"
)

// ContentBlock is a single block within a message.
type ContentBlock struct {
	Type      ContentType `json:"type"`
	Text      string      `json:"text,omitempty"`
	Thinking  string      `json:"thinking,omitempty"`
	Signature string      `json:"signature,omitempty"`

	// Image
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`

	// Tool use
	ToolUseID string      `json:"id,omitempty"`
	ToolName  string      `json:"name,omitempty"`
	Input     interface{} `json:"input,omitempty"`

	// Tool result
	Content []*ContentBlock `json:"content,omitempty"`
	IsError bool            `json:"is_error,omitempty"`

	// Cache control (prompt caching)
	CacheControl *CacheControl `json:"cache_control,omitempty"`

	// Redacted thinking (returned by model when thinking is redacted)
	RedactedData string `json:"data_redacted,omitempty"`
}

// CacheControl specifies caching behavior for a content block.
type CacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

// MessageType discriminates different message subtypes within the same Role.
type MessageType string

const (
	MsgTypeUser                  MessageType = "user"
	MsgTypeAssistant             MessageType = "assistant"
	MsgTypeSystem                MessageType = "system"
	MsgTypeProgress              MessageType = "progress"
	MsgTypeAttachment            MessageType = "attachment"
	MsgTypeTombstone             MessageType = "tombstone"
	MsgTypeToolUseSummary        MessageType = "tool_use_summary"
	MsgTypeCompactBoundary       MessageType = "compact_boundary"
	MsgTypeMicrocompactBoundary  MessageType = "microcompact_boundary"
	MsgTypeSystemAPIError        MessageType = "system_api_error"
	MsgTypeSystemInformational   MessageType = "system_informational"
	MsgTypeSystemLocalCommand    MessageType = "system_local_command"
	MsgTypeSystemMemorySaved     MessageType = "system_memory_saved"
	MsgTypeSystemPermissionRetry MessageType = "system_permission_retry"
	MsgTypeSystemStopHookSummary MessageType = "system_stop_hook_summary"
	MsgTypeSystemTurnDuration    MessageType = "system_turn_duration"
)

// SystemMessageLevel indicates the severity of a system message.
type SystemMessageLevel string

const (
	SystemLevelInfo    SystemMessageLevel = "info"
	SystemLevelWarning SystemMessageLevel = "warning"
	SystemLevelError   SystemMessageLevel = "error"
)

// MessageOrigin tracks where a message was created.
type MessageOrigin string

const (
	OriginUser      MessageOrigin = "user"
	OriginAssistant MessageOrigin = "assistant"
	OriginSystem    MessageOrigin = "system"
	OriginTool      MessageOrigin = "tool"
	OriginHook      MessageOrigin = "hook"
	OriginCompact   MessageOrigin = "compact"
)

// Message is the canonical internal representation of a conversation turn.
type Message struct {
	ID        string          `json:"id,omitempty"`
	UUID      string          `json:"uuid,omitempty"`
	Role      MessageRole     `json:"role"`
	Type      MessageType     `json:"type,omitempty"`
	Content   []*ContentBlock `json:"content"`
	Timestamp time.Time       `json:"timestamp,omitempty"`

	// SessionID links the message to a session for persistence.
	SessionID string `json:"session_id,omitempty"`

	// ── User message metadata ────────────────────────────────────────────
	IsMeta                    bool   `json:"is_meta,omitempty"`
	IsVisibleInTranscriptOnly bool   `json:"is_visible_in_transcript_only,omitempty"`
	ToolUseResult             string `json:"tool_use_result,omitempty"`
	SourceToolAssistantUUID   string `json:"source_tool_assistant_uuid,omitempty"`

	// ── Assistant message metadata ───────────────────────────────────────
	CostUSD   float64     `json:"cost_usd,omitempty"`
	Usage     *UsageStats `json:"usage,omitempty"`
	IsVirtual bool        `json:"is_virtual,omitempty"` // not from API (e.g. synthetic error)
	Model     string      `json:"model,omitempty"`

	// ── System/error metadata ────────────────────────────────────────────
	Level    SystemMessageLevel `json:"level,omitempty"`
	APIError string             `json:"api_error,omitempty"` // "invalid_request", "overloaded", etc.
	Error    string             `json:"error,omitempty"`

	// ── Compact metadata ────────────────────────────────────────────────
	IsCompactSummary bool `json:"is_compact_summary,omitempty"`

	// ── Progress metadata ───────────────────────────────────────────────
	ProgressData *ProgressData `json:"progress_data,omitempty"`

	// ── Attachment metadata ─────────────────────────────────────────────
	Attachment *AttachmentData `json:"attachment,omitempty"`

	// ── Tombstone metadata ──────────────────────────────────────────────
	TombstoneFor string `json:"tombstone_for,omitempty"` // UUID of message to remove

	// ── ToolUseSummary metadata ─────────────────────────────────────────
	ToolUseSummary *ToolUseSummaryData `json:"tool_use_summary,omitempty"`

	// ── Origin tracking ─────────────────────────────────────────────────
	Origin     MessageOrigin `json:"origin,omitempty"`
	StopReason string        `json:"stop_reason,omitempty"` // "end_turn", "tool_use", "max_tokens", "stop_sequence"

	// ── Stop hook info ──────────────────────────────────────────────────
	StopHookInfo *StopHookInfo `json:"stop_hook_info,omitempty"`
}

// StopHookInfo carries metadata when a stop hook fires.
type StopHookInfo struct {
	HookName      string `json:"hook_name"`
	Passed        bool   `json:"passed"`
	FailureReason string `json:"failure_reason,omitempty"`
	DurationMs    int    `json:"duration_ms,omitempty"`
}

// ToolCall represents a request for a tool invocation from the LLM.
type ToolCall struct {
	ID    string      `json:"id"`
	Name  string      `json:"name"`
	Input interface{} `json:"input"`
}

// ToolResult carries the outcome of a tool invocation.
type ToolResult struct {
	ToolUseID string          `json:"tool_use_id"`
	Content   []*ContentBlock `json:"content"`
	IsError   bool            `json:"is_error,omitempty"`
}

// EngineConfig holds all configuration needed to create an Engine instance.
type EngineConfig struct {
	// Provider selection: "anthropic" or "openai"
	Provider string `json:"provider" validate:"required,oneof=anthropic openai"`

	// API credentials
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url,omitempty"`

	// Model settings
	Model          string  `json:"model"`
	MaxTokens      int     `json:"max_tokens"`
	ThinkingBudget int     `json:"thinking_budget,omitempty"`
	Temperature    float64 `json:"temperature,omitempty"`

	// Working directory
	WorkDir string `json:"work_dir"`

	// Session
	SessionID string `json:"session_id,omitempty"`

	// Feature flags
	AutoMode   bool    `json:"auto_mode,omitempty"`
	FastMode   bool    `json:"fast_mode,omitempty"`
	MaxCostUSD float64 `json:"max_cost_usd,omitempty"`

	// Permission mode: "default", "auto", "bypass", "plan", "acceptEdits", "dontAsk"
	PermissionMode string `json:"permission_mode,omitempty"`

	// Fallback model when the primary model is unavailable.
	FallbackModel string `json:"fallback_model,omitempty"`

	// System prompt overrides
	CustomSystemPrompt string `json:"custom_system_prompt,omitempty"`
	AppendSystemPrompt string `json:"append_system_prompt,omitempty"`

	// Non-interactive session (no user prompts allowed, e.g. CI/CD).
	NonInteractive bool `json:"non_interactive,omitempty"`

	// Output style: "concise", "verbose", "code-only", etc.
	OutputStyle string `json:"output_style,omitempty"`

	// Effort value for model calls (low/medium/high).
	EffortValue string `json:"effort_value,omitempty"`

	// Verbose / debug
	Verbose bool `json:"verbose,omitempty"`

	// AdditionalWorkingDirs are extra directories the session may access.
	AdditionalWorkingDirs []string `json:"additional_working_dirs,omitempty"`
}

// QuerySource describes the origin of a query (for compaction skip decisions).
type QuerySource string

const (
	QuerySourceUser          QuerySource = "user"
	QuerySourceCompact       QuerySource = "compact"
	QuerySourceSessionMemory QuerySource = "session_memory"
	QuerySourceAgent         QuerySource = "agent"
	QuerySourceSkill         QuerySource = "skill"
)

// QueryParams contains per-request parameters for Engine.SubmitMessage.
type QueryParams struct {
	// Content of the user message.
	Text   string
	Images []string // base64-encoded images

	// Config carries per-query tuning knobs (overrides engine-level EngineConfig).
	Config QueryConfig
	// Deps carries optional per-query dependency overrides.
	Deps QueryDeps

	// Source indicates the origin of this query (for compaction/routing decisions).
	Source QuerySource
	// TaskBudget is the total token budget for a task-based query.
	TaskBudget *TaskBudget
	// JSONSchema, if non-nil, forces structured (JSON) output from the model.
	JSONSchema json.RawMessage
}

// TaskBudget limits the total tokens a task may consume.
type TaskBudget struct {
	Total     int `json:"total"`
	Remaining int `json:"remaining,omitempty"`
}

// ToolChoice controls which tool the model should use.
type ToolChoice struct {
	Type string `json:"type"` // "auto", "any", "tool"
	Name string `json:"name,omitempty"`
}
