package brief

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/state"
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
// Renamed to SendUserMessage to align with claude-code-main BriefTool.
type BriefTool struct{ tool.BaseTool }

func New() *BriefTool { return &BriefTool{} }

func (t *BriefTool) Name() string                             { return "SendUserMessage" }
func (t *BriefTool) Aliases() []string                        { return []string{"Brief"} }
func (t *BriefTool) UserFacingName() string                   { return "send_user_message" }
func (t *BriefTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (t *BriefTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *BriefTool) MaxResultSizeChars() int                  { return 10_000 }

func (t *BriefTool) Description() string {
	return "Send a message to the user with optional file attachments. " +
		"Use status='proactive' for unsolicited updates in daemon mode."
}

// IsEnabled checks KairosActive or UserMsgOptIn from AppState.
// Aligned with claude-code-main BriefTool.ts isBriefEnabled().
func (t *BriefTool) IsEnabled(uctx *tool.UseContext) bool {
	if uctx == nil || uctx.GetAppState == nil {
		return true // default enabled when no state available
	}
	v := uctx.GetAppState()
	as, ok := v.(*state.AppState)
	if !ok {
		return true
	}
	return as.KairosActive || as.UserMsgOptIn
}

func (t *BriefTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"content":{"type":"string","description":"Message content (Markdown supported)."},
			"format":{"type":"string","enum":["markdown","text"],"description":"Format hint for the UI."},
			"attachments":{"type":"array","items":{"type":"string"},"description":"Optional file paths to attach."},
			"status":{"type":"string","enum":["normal","proactive"],"description":"Message type. normal: reply. proactive: unsolicited update."}
		},
		"required":["content"]
	}`)
}

func (t *BriefTool) Prompt(_ *tool.UseContext) string {
	return `Send a message to the user with optional file attachments.

Use this tool to:
- Provide concise status updates or summaries to the user
- Share proactive observations or recommendations
- Attach relevant files to your message
- Deliver structured information in a collapsible format

The status field controls how the message is displayed:
- normal: Standard reply shown inline
- proactive: Unsolicited information shown with a distinct indicator

When in assistant daemon mode, proactive messages are the primary way to
communicate findings or updates to the user between scheduled tasks.`
}

func (t *BriefTool) CheckPermissions(_ context.Context, input json.RawMessage, uctx *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Content == "" {
		return fmt.Errorf("content must not be empty")
	}
	// Validate attachment paths
	if len(in.Attachments) > 0 {
		cwd := ""
		if uctx != nil {
			cwd = uctx.WorkDir
		}
		if err := validateAttachmentPaths(in.Attachments, cwd); err != nil {
			return err
		}
	}
	return nil
}

func (t *BriefTool) Call(_ context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 2+len(in.Attachments))
	go func() {
		defer close(ch)

		sentAt := time.Now().UTC().Format(time.RFC3339)
		text := in.Content + "\n\n_Sent at " + sentAt + "_"
		if in.Status == "proactive" {
			text = "[Proactive] " + text
		}

		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: text}

		// Emit resolved attachment info.
		cwd := ""
		if uctx != nil {
			cwd = uctx.WorkDir
		}
		for _, path := range in.Attachments {
			resolved := resolveAttachment(path, cwd)
			ch <- &engine.ContentBlock{
				Type: engine.ContentTypeText,
				Text: resolved,
			}
		}
	}()
	return ch, nil
}

// MapToolResultToBlockParam returns the message with attachment count metadata.
func (t *BriefTool) MapToolResultToBlockParam(content interface{}, toolUseID string) *engine.ContentBlock {
	text, ok := content.(string)
	if !ok {
		return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: ""}
	}
	return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: text}
}

// ─── Attachment helpers ─────────────────────────────────────────────────────

var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".bmp": true, ".webp": true, ".svg": true, ".ico": true,
}

func validateAttachmentPaths(paths []string, cwd string) error {
	for _, p := range paths {
		abs := p
		if !filepath.IsAbs(p) && cwd != "" {
			abs = filepath.Join(cwd, p)
		}
		if _, err := os.Stat(abs); err != nil {
			return fmt.Errorf("attachment not found: %s", p)
		}
	}
	return nil
}

func resolveAttachment(path, cwd string) string {
	abs := path
	if !filepath.IsAbs(path) && cwd != "" {
		abs = filepath.Join(cwd, path)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Sprintf("[Attachment: %s (not found)]", path)
	}

	ext := strings.ToLower(filepath.Ext(abs))
	kind := "file"
	if imageExtensions[ext] {
		kind = "image"
	}

	return fmt.Sprintf("[Attachment: %s (%s, %d bytes)]", path, kind, info.Size())
}
