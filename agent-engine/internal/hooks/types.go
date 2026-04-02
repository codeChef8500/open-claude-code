package hooks

import (
	"encoding/json"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// HookEvent enumerates all lifecycle hook events.
// Aligned with claude-code-main types/hooks.ts HookEventName.
// ────────────────────────────────────────────────────────────────────────────

// HookEvent identifies which lifecycle point the hook fires at.
type HookEvent string

const (
	EventPreToolUse         HookEvent = "PreToolUse"
	EventPostToolUse        HookEvent = "PostToolUse"
	EventNotification       HookEvent = "Notification"
	EventStop               HookEvent = "Stop"
	EventStopFailure        HookEvent = "StopFailure"
	EventPermissionDenied   HookEvent = "PermissionDenied"
	EventPreCompact         HookEvent = "PreCompact"
	EventPostCompact        HookEvent = "PostCompact"
	EventSessionStart       HookEvent = "SessionStart"
	EventSessionEnd         HookEvent = "SessionEnd"
	EventSetup              HookEvent = "Setup"
	EventSubagentStart      HookEvent = "SubagentStart"
	EventSubagentStop       HookEvent = "SubagentStop"
	EventTeammateIdle       HookEvent = "TeammateIdle"
	EventTaskCreated        HookEvent = "TaskCreated"
	EventTaskCompleted      HookEvent = "TaskCompleted"
	EventConfigChange       HookEvent = "ConfigChange"
	EventCwdChanged         HookEvent = "CwdChanged"
	EventFileChanged        HookEvent = "FileChanged"
	EventInstructionsLoaded HookEvent = "InstructionsLoaded"
	EventUserPromptSubmit   HookEvent = "UserPromptSubmit"
	EventPostSampling       HookEvent = "PostSampling"
)

// AllHookEvents is the complete list of hook events.
var AllHookEvents = []HookEvent{
	EventPreToolUse, EventPostToolUse, EventNotification,
	EventStop, EventStopFailure, EventPermissionDenied,
	EventPreCompact, EventPostCompact,
	EventSessionStart, EventSessionEnd, EventSetup,
	EventSubagentStart, EventSubagentStop, EventTeammateIdle,
	EventTaskCreated, EventTaskCompleted,
	EventConfigChange, EventCwdChanged, EventFileChanged,
	EventInstructionsLoaded, EventUserPromptSubmit, EventPostSampling,
}

// ────────────────────────────────────────────────────────────────────────────
// Hook input types — per-event payloads passed to hook handlers.
// ────────────────────────────────────────────────────────────────────────────

// HookInput is the base payload sent to all hooks (JSON-serialized to stdin
// for external hook scripts).
type HookInput struct {
	// Event is the hook event name.
	Event HookEvent `json:"hook_event_name"`

	// SessionID is the active session identifier.
	SessionID string `json:"session_id"`

	// CWD is the current working directory.
	CWD string `json:"cwd"`

	// Timestamp of the event.
	Timestamp time.Time `json:"timestamp"`

	// Per-event data (only the relevant field is populated).
	PreToolUse       *PreToolUseInput       `json:"pre_tool_use,omitempty"`
	PostToolUse      *PostToolUseInput      `json:"post_tool_use,omitempty"`
	Stop             *StopInput             `json:"stop,omitempty"`
	Notification     *NotificationInput     `json:"notification,omitempty"`
	PermissionDenied *PermissionDeniedInput `json:"permission_denied,omitempty"`
	PostSampling     *PostSamplingInput     `json:"post_sampling,omitempty"`
	PreCompact       *PreCompactInput       `json:"pre_compact,omitempty"`
	FileChanged      *FileChangedInput      `json:"file_changed,omitempty"`
	ConfigChange     *ConfigChangeInput     `json:"config_change,omitempty"`
	Task             *TaskInput             `json:"task,omitempty"`
}

// PreToolUseInput carries the data for a PreToolUse hook.
type PreToolUseInput struct {
	ToolName string          `json:"tool_name"`
	ToolID   string          `json:"tool_use_id"`
	Input    json.RawMessage `json:"input"`
}

// PostToolUseInput carries the data for a PostToolUse hook.
type PostToolUseInput struct {
	ToolName string          `json:"tool_name"`
	ToolID   string          `json:"tool_use_id"`
	Input    json.RawMessage `json:"input"`
	Output   string          `json:"output"`
	IsError  bool            `json:"is_error"`
}

// StopInput carries the data for a Stop hook.
type StopInput struct {
	// StopReason is the model's stop_reason ("end_turn", "tool_use", etc.).
	StopReason string `json:"stop_reason"`
	// AssistantMessage is the last assistant message text.
	AssistantMessage string `json:"assistant_message,omitempty"`
}

// NotificationInput carries the data for a Notification hook.
type NotificationInput struct {
	Message string `json:"message"`
}

// PermissionDeniedInput carries the data for a PermissionDenied hook.
type PermissionDeniedInput struct {
	ToolName string `json:"tool_name"`
	Reason   string `json:"reason"`
	RuleType string `json:"rule_type,omitempty"` // "auto_classifier", "rule_based", "hook"
}

// PostSamplingInput carries the data for a PostSampling hook.
type PostSamplingInput struct {
	// AssistantContent is the raw content blocks from the model response.
	AssistantContent json.RawMessage `json:"assistant_content"`
	StopReason       string          `json:"stop_reason"`
}

// FileChangedInput carries the data for a FileChanged hook.
type FileChangedInput struct {
	FilePath   string `json:"file_path"`
	ChangeType string `json:"change_type"` // "created", "modified", "deleted"
}

// ConfigChangeInput carries the data for a ConfigChange hook.
type ConfigChangeInput struct {
	Key      string `json:"key"`
	OldValue string `json:"old_value,omitempty"`
	NewValue string `json:"new_value"`
}

// PreCompactInput carries the data for a PreCompact hook.
type PreCompactInput struct {
	Trigger            string `json:"trigger"` // "auto" or "manual"
	CustomInstructions string `json:"custom_instructions,omitempty"`
}

// TaskInput carries the data for TaskCreated/TaskCompleted hooks.
type TaskInput struct {
	TaskID      string `json:"task_id"`
	Title       string `json:"title,omitempty"`
	Status      string `json:"status,omitempty"`
	Description string `json:"description,omitempty"`
}

// ────────────────────────────────────────────────────────────────────────────
// Hook output types — the JSON structure hooks return via stdout.
// ────────────────────────────────────────────────────────────────────────────

// HookJSONOutput is the raw JSON object that an external hook script writes
// to stdout. The engine parses this to determine what action to take.
type HookJSONOutput struct {
	// ── Common fields ──────────────────────────────────────────────────
	// Continue indicates whether to proceed (true) or stop/block (false).
	Continue *bool `json:"continue,omitempty"`

	// ShouldStop, if true, signals the query loop to stop.
	ShouldStop bool `json:"shouldStop,omitempty"`

	// StopReason is a human-readable reason for stopping.
	StopReason string `json:"stopReason,omitempty"`

	// ── PreToolUse response fields ─────────────────────────────────────
	// Decision is the permission outcome: "approve", "block", "ask".
	Decision string `json:"decision,omitempty"`

	// Reason is the rationale for the decision (shown to user and model).
	Reason string `json:"reason,omitempty"`

	// UpdatedInput replaces the tool's input when present.
	UpdatedInput json.RawMessage `json:"updatedInput,omitempty"`

	// AdditionalContext is injected as a system message before the tool call.
	AdditionalContext string `json:"additionalContext,omitempty"`

	// ── PostToolUse response fields ────────────────────────────────────
	// OutputOverride replaces the tool result when present.
	OutputOverride *string `json:"outputOverride,omitempty"`

	// ── Stop hook response fields ──────────────────────────────────────
	// Passed is true if the stop hook's condition was satisfied.
	Passed *bool `json:"passed,omitempty"`

	// FailureReason is populated when Passed is false.
	FailureReason string `json:"failureReason,omitempty"`

	// ── PreCompact hook response fields ────────────────────────────────
	// NewCustomInstructions are additional instructions from hooks.
	NewCustomInstructions string `json:"newCustomInstructions,omitempty"`

	// ── Prompt elicitation (interactive hooks) ─────────────────────────
	// Prompt requests user input during hook execution.
	Prompt *PromptRequest `json:"prompt,omitempty"`
}

// PromptRequest is sent by a hook to request interactive user input.
type PromptRequest struct {
	// Title is displayed as the prompt header.
	Title string `json:"title"`

	// Message is the body text explaining what input is needed.
	Message string `json:"message"`

	// Options are the available choices (if empty, free text is expected).
	Options []PromptOption `json:"options,omitempty"`

	// AllowFreeText allows the user to type an arbitrary response.
	AllowFreeText bool `json:"allowFreeText,omitempty"`

	// Timeout is the maximum seconds to wait for a response (0 = no timeout).
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
}

// PromptOption is a single selectable choice in a PromptRequest.
type PromptOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// PromptResponse is the user's reply to a PromptRequest.
type PromptResponse struct {
	// Selected is the value of the chosen option.
	Selected string `json:"selected,omitempty"`

	// FreeText is the user's free-text input.
	FreeText string `json:"freeText,omitempty"`

	// TimedOut is true if the prompt was not answered before the deadline.
	TimedOut bool `json:"timedOut,omitempty"`
}

// ────────────────────────────────────────────────────────────────────────────
// Sync vs Async hook response wrappers
// ────────────────────────────────────────────────────────────────────────────

// SyncHookResponse is the fully-parsed result of running a synchronous hook.
type SyncHookResponse struct {
	// Decision is the permission decision (for PreToolUse hooks).
	Decision string // "approve", "block", "ask", ""

	// ShouldStop tells the query loop to stop.
	ShouldStop bool

	// StopReason is the human-readable stop reason.
	StopReason string

	// UpdatedInput, if non-nil, replaces the tool input.
	UpdatedInput json.RawMessage

	// AdditionalContext is injected as a system message.
	AdditionalContext string

	// OutputOverride replaces the tool result (PostToolUse only).
	OutputOverride *string

	// Passed is the stop-hook pass/fail verdict.
	Passed *bool

	// FailureReason is the stop-hook failure explanation.
	FailureReason string

	// NewCustomInstructions are additional instructions from PreCompact hooks.
	NewCustomInstructions string

	// Error is set if the hook itself failed (script error, timeout, etc.).
	Error error
}

// ────────────────────────────────────────────────────────────────────────────
// Hook configuration (what gets loaded from settings.json)
// ────────────────────────────────────────────────────────────────────────────

// HookConfig describes a single hook entry from the configuration file.
type HookConfig struct {
	// Event is the lifecycle event this hook listens to.
	Event HookEvent `json:"event"`

	// Command is the shell command to execute.
	Command string `json:"command"`

	// Args are additional arguments passed to the command.
	Args []string `json:"args,omitempty"`

	// Timeout is the maximum execution time in seconds (0 = default 60s).
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`

	// Async, if true, runs the hook without blocking the engine.
	Async bool `json:"async,omitempty"`

	// Env is additional environment variables for the hook process.
	Env map[string]string `json:"env,omitempty"`

	// Source identifies who registered this hook (user, project, plugin).
	Source string `json:"source,omitempty"`
}

// HooksSettings is the "hooks" section of settings.json, mapping event names
// to lists of hook configurations.
type HooksSettings map[HookEvent][]HookConfig
