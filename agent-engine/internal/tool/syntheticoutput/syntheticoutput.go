package syntheticoutput

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// ────────────────────────────────────────────────────────────────────────────
// SyntheticOutputTool — injects synthetic assistant output into the conversation.
// Used by hooks and internal systems to add messages that appear as if they
// came from the assistant, for progress updates, status messages, etc.
// Aligned with claude-code-main's synthetic output injection pattern.
// ────────────────────────────────────────────────────────────────────────────

// Input is the JSON input schema for SyntheticOutputTool.
type Input struct {
	// Text is the synthetic text to inject.
	Text string `json:"text"`
	// Role controls who the message appears from ("assistant", "system").
	Role string `json:"role,omitempty"`
	// Format controls output formatting ("text", "markdown", "json").
	Format string `json:"format,omitempty"`
	// Ephemeral if true means the message should not be persisted to history.
	Ephemeral bool `json:"ephemeral,omitempty"`
}

// SyntheticOutputTool injects synthetic content into the conversation stream.
type SyntheticOutputTool struct{ tool.BaseTool }

func New() *SyntheticOutputTool { return &SyntheticOutputTool{} }

func (t *SyntheticOutputTool) Name() string           { return "SyntheticOutput" }
func (t *SyntheticOutputTool) UserFacingName() string { return "synthetic_output" }
func (t *SyntheticOutputTool) Description() string {
	return "Inject synthetic content into the conversation (used by hooks and internal systems)."
}
func (t *SyntheticOutputTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (t *SyntheticOutputTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *SyntheticOutputTool) MaxResultSizeChars() int                  { return 200_000 }
func (t *SyntheticOutputTool) IsEnabled(uctx *tool.UseContext) bool     { return true }
func (t *SyntheticOutputTool) Aliases() []string                        { return []string{"synthetic_output", "inject"} }
func (t *SyntheticOutputTool) IsTransparentWrapper() bool               { return true }

func (t *SyntheticOutputTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"text":{"type":"string","description":"The synthetic text content to inject."},
			"role":{"type":"string","description":"Message role (assistant or system).","enum":["assistant","system"]},
			"format":{"type":"string","description":"Output format.","enum":["text","markdown","json"]},
			"ephemeral":{"type":"boolean","description":"If true, message is not persisted to history."}
		},
		"required":["text"]
	}`)
}

func (t *SyntheticOutputTool) Prompt(uctx *tool.UseContext) string {
	return `## SyntheticOutput
Inject synthetic content into the conversation. Used internally by hooks
and system processes. Not intended for direct use by the assistant.`
}

func (t *SyntheticOutputTool) CheckPermissions(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Text == "" {
		return fmt.Errorf("text must not be empty")
	}
	return nil
}

func (t *SyntheticOutputTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	role := in.Role
	if role == "" {
		role = "system"
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)

		// If there's an AppendSystemMessage callback, use it for system messages.
		if role == "system" && uctx != nil && uctx.AppendSystemMessage != nil {
			msg := &engine.Message{
				Role: engine.MessageRole(role),
				Content: []*engine.ContentBlock{{
					Type: engine.ContentTypeText,
					Text: in.Text,
				}},
			}
			uctx.AppendSystemMessage(msg)
			ch <- &engine.ContentBlock{
				Type: engine.ContentTypeText,
				Text: fmt.Sprintf("[Synthetic %s message injected]", role),
			}
			return
		}

		// Otherwise, return the text as a tool result.
		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: in.Text,
		}
	}()
	return ch, nil
}

func (t *SyntheticOutputTool) GetActivityDescription(_ json.RawMessage) string {
	return "Injecting synthetic output"
}
