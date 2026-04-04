package brief

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

type Input struct {
	Content     string   `json:"content"`
	Format      string   `json:"format,omitempty"`      // "markdown" | "text"
	Attachments []string `json:"attachments,omitempty"` // file paths to attach
	Status      string   `json:"status,omitempty"`      // "normal" | "proactive"
}

// BriefTool emits a structured brief/summary block that callers can render
// specially in their UI (e.g. collapsible panel).
type BriefTool struct{ tool.BaseTool }

func New() *BriefTool { return &BriefTool{} }

func (t *BriefTool) Name() string                             { return "Brief" }
func (t *BriefTool) UserFacingName() string                   { return "brief" }
func (t *BriefTool) Description() string                      { return "Emit a structured brief or progress summary." }
func (t *BriefTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (t *BriefTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *BriefTool) MaxResultSizeChars() int                  { return 10_000 }
func (t *BriefTool) IsEnabled(_ *tool.UseContext) bool        { return true }

func (t *BriefTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"content":{"type":"string","description":"Brief content (Markdown supported)."},
			"format":{"type":"string","enum":["markdown","text"],"description":"Format hint for the UI."},
			"attachments":{"type":"array","items":{"type":"string"},"description":"Optional file paths to attach to the brief."},
			"status":{"type":"string","enum":["normal","proactive"],"description":"Brief status type. normal: standard update. proactive: unsolicited information."}
		},
		"required":["content"]
	}`)
}

func (t *BriefTool) Prompt(_ *tool.UseContext) string {
	return `Emit a structured brief or progress summary that callers can render specially in their UI (e.g. collapsible panel).

Use this tool to:
- Provide concise status updates or summaries to the user
- Share proactive observations or recommendations
- Attach relevant files to your message
- Deliver structured information in a collapsible format

The status field controls how the brief is displayed:
- normal: Standard update shown inline
- proactive: Unsolicited information shown with a distinct indicator`
}

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

	ch := make(chan *engine.ContentBlock, 2+len(in.Attachments))
	go func() {
		defer close(ch)

		// Add timestamp.
		sentAt := time.Now().UTC().Format(time.RFC3339)
		text := in.Content + "\n\n_Sent at " + sentAt + "_"
		if in.Status == "proactive" {
			text = "[Proactive] " + text
		}

		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: text}

		// Emit attachment references.
		for _, path := range in.Attachments {
			ch <- &engine.ContentBlock{
				Type: engine.ContentTypeText,
				Text: fmt.Sprintf("[Attachment: %s]", path),
			}
		}
	}()
	return ch, nil
}
