package permission

import "context"

// ClassifierResult is the outcome of an auto-mode security classifier.
// Aligned with claude-code-main's YoloClassifierResult.
type ClassifierResult struct {
	ShouldBlock bool   `json:"should_block"`
	Reason      string `json:"reason"`
	Model       string `json:"model,omitempty"`
	DurationMs  int64  `json:"duration_ms,omitempty"`

	// Usage tracks token consumption by the classifier call.
	Usage *ClassifierUsage `json:"usage,omitempty"`

	// Stage indicates which classifier stage produced the decision.
	// Values: "fast", "thinking", "both", "rule_based".
	Stage string `json:"stage,omitempty"`

	// PromptLengths records the sizes of the classifier's input sections.
	PromptLengths *ClassifierPromptLengths `json:"prompt_lengths,omitempty"`
}

// ClassifierUsage tracks token counts consumed by a classifier call.
type ClassifierUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

// ClassifierPromptLengths records the sizes of the classifier prompt sections.
type ClassifierPromptLengths struct {
	SystemPrompt int `json:"system_prompt"`
	ToolCalls    int `json:"tool_calls"`
	UserPrompts  int `json:"user_prompts"`
}

// AutoModeRules defines the user-configurable auto-mode classifier sections.
// Aligned with claude-code-main's AutoModeRules.
type AutoModeRules struct {
	Allow       []string `json:"allow"`
	SoftDeny    []string `json:"soft_deny"`
	Environment []string `json:"environment"`
}

// Classifier is the interface for an auto-mode security classifier.
// The classifier evaluates whether a tool use should be allowed or blocked
// based on the conversation transcript and tool input.
type Classifier interface {
	// Classify evaluates a tool action and returns the classification result.
	Classify(ctx context.Context, req ClassifyRequest) (*ClassifierResult, error)
}

// ClassifyRequest carries all information needed for the classifier.
type ClassifyRequest struct {
	// ToolName is the name of the tool being invoked.
	ToolName string

	// ToolInput is the tool's input parameters.
	ToolInput interface{}

	// Action is the compact classifier representation of the tool use
	// (from Tool.ToAutoClassifierInput).
	Action string

	// Transcript is the compact conversation transcript for the classifier.
	Transcript string

	// PermissionContext carries the current permission rules and state.
	PermissionContext *ToolPermissionContext

	// Model is the model to use for classification (may differ from main loop).
	Model string

	// Signal allows cancellation of the classifier call.
	Signal context.Context
}

// PermissionDecision represents the full decision outcome of a permission check.
// Aligned with claude-code-main's PermissionDecision union.
type PermissionDecision struct {
	// Type is one of: "allow", "deny", "ask".
	Type string `json:"type"`

	// Reason explains why this decision was made.
	Reason *PermissionDecisionReason `json:"reason,omitempty"`

	// UpdatedInput is the potentially-modified tool input (hooks may modify it).
	UpdatedInput interface{} `json:"updated_input,omitempty"`

	// Message is displayed to the user (for ask/deny decisions).
	Message string `json:"message,omitempty"`

	// AllowUpdates are permission rule updates to apply if the user approves.
	AllowUpdates []PermissionUpdate `json:"allow_updates,omitempty"`
}

// PermissionDecisionReason describes why a permission decision was made.
type PermissionDecisionReason struct {
	// Type is one of: "rule", "hook", "classifier", "mode", "tool_specific".
	Type string `json:"type"`

	// Source identifies the specific rule/hook/classifier that made the decision.
	Source string `json:"source,omitempty"`

	// Classifier identifies which classifier made the decision (for type="classifier").
	Classifier string `json:"classifier,omitempty"`

	// Reason is a human-readable explanation.
	Reason string `json:"reason,omitempty"`
}

// NoopClassifier always allows all actions (used when auto-mode is disabled).
type NoopClassifier struct{}

// Classify always returns a non-blocking result.
func (n *NoopClassifier) Classify(_ context.Context, _ ClassifyRequest) (*ClassifierResult, error) {
	return &ClassifierResult{
		ShouldBlock: false,
		Reason:      "auto-mode classifier disabled",
		Stage:       "noop",
	}, nil
}
