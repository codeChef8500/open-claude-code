package engine

import "time"

// StreamEventType enumerates all event types emitted by the engine.
type StreamEventType string

const (
	EventTextDelta      StreamEventType = "text_delta"
	EventTextComplete   StreamEventType = "text_complete"
	EventToolUse        StreamEventType = "tool_use"
	EventToolResult     StreamEventType = "tool_result"
	EventThinking       StreamEventType = "thinking"
	EventUsage          StreamEventType = "usage"
	EventError          StreamEventType = "error"
	EventDone           StreamEventType = "done"
	EventSystemMessage  StreamEventType = "system_message"
)

// StreamEvent is produced by the engine and consumed by SDK callers or HTTP SSE.
type StreamEvent struct {
	Type       StreamEventType `json:"type"`
	Text       string          `json:"text,omitempty"`
	ToolName   string          `json:"tool_name,omitempty"`
	ToolID     string          `json:"tool_id,omitempty"`
	ToolInput  interface{}     `json:"tool_input,omitempty"`
	Result     string          `json:"result,omitempty"`
	IsError    bool            `json:"is_error,omitempty"`
	Thinking   string          `json:"thinking,omitempty"`
	Usage      *UsageStats     `json:"usage,omitempty"`
	Error      string          `json:"error,omitempty"`
	SessionID  string          `json:"session_id,omitempty"`
}

// UsageStats carries token and cost information from an LLM response.
type UsageStats struct {
	InputTokens              int     `json:"input_tokens"`
	OutputTokens             int     `json:"output_tokens"`
	CacheCreationInputTokens int     `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int     `json:"cache_read_input_tokens,omitempty"`
	CostUSD                  float64 `json:"cost_usd,omitempty"`
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
	ContentTypeText        ContentType = "text"
	ContentTypeImage       ContentType = "image"
	ContentTypeToolUse     ContentType = "tool_use"
	ContentTypeToolResult  ContentType = "tool_result"
	ContentTypeThinking    ContentType = "thinking"
	ContentTypeDocument    ContentType = "document"
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
	Content   []*ContentBlock `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

// Message is the canonical internal representation of a conversation turn.
type Message struct {
	ID        string          `json:"id,omitempty"`
	Role      MessageRole     `json:"role"`
	Content   []*ContentBlock `json:"content"`
	Timestamp time.Time       `json:"timestamp,omitempty"`
	// SessionID links the message to a session for persistence.
	SessionID string `json:"session_id,omitempty"`
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
	AutoMode       bool   `json:"auto_mode,omitempty"`
	FastMode       bool   `json:"fast_mode,omitempty"`
	MaxCostUSD     float64 `json:"max_cost_usd,omitempty"`

	// System prompt overrides
	CustomSystemPrompt string `json:"custom_system_prompt,omitempty"`
	AppendSystemPrompt string `json:"append_system_prompt,omitempty"`

	// Verbose / debug
	Verbose bool `json:"verbose,omitempty"`
}

// QueryParams contains per-request parameters for Engine.SubmitMessage.
type QueryParams struct {
	// Content of the user message.
	Text   string
	Images []string // base64-encoded images

	// Optional per-request overrides.
	Model     string
	MaxTokens int
}

// SessionState holds mutable per-session state shared across the query loop.
type SessionState struct {
	SessionID  string
	WorkDir    string
	TotalCost  float64
	TurnCount  int
	Messages   []*Message
	Compacted  bool
}
