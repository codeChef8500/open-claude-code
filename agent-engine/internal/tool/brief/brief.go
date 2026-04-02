package brief

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

type Input struct {
	Content string `json:"content"`
	Format  string `json:"format,omitempty"` // "markdown" | "text"
}

// BriefTool emits a structured brief/summary block that callers can render
// specially in their UI (e.g. collapsible panel).
type BriefTool struct{ tool.BaseTool }

func New() *BriefTool { return &BriefTool{} }

func (t *BriefTool) Name() string                      { return "Brief" }
func (t *BriefTool) UserFacingName() string            { return "brief" }
func (t *BriefTool) Description() string               { return "Emit a structured brief or progress summary." }
func (t *BriefTool) IsReadOnly() bool                  { return true }
func (t *BriefTool) IsConcurrencySafe() bool           { return true }
func (t *BriefTool) MaxResultSizeChars() int           { return 10_000 }
func (t *BriefTool) IsEnabled(_ *tool.UseContext) bool { return true }

func (t *BriefTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"content":{"type":"string","description":"Brief content (Markdown supported)."},
			"format":{"type":"string","enum":["markdown","text"],"description":"Format hint for the UI."}
		},
		"required":["content"]
	}`)
}

func (t *BriefTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *BriefTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Content == "" {
		return fmt.Errorf("content must not be empty")
	}
	return nil
}

func (t *BriefTool) Call(_ context.Context, input json.RawMessage, _ *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: in.Content}
	}()
	return ch, nil
}
