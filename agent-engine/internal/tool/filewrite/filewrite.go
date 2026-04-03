package filewrite

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
	"github.com/wall-ai/agent-engine/internal/tool/fileread"
	"github.com/wall-ai/agent-engine/internal/util"
)

type Input struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type FileWriteTool struct{ tool.BaseTool }

func New() *FileWriteTool { return &FileWriteTool{} }

func (t *FileWriteTool) Name() string                             { return "Write" }
func (t *FileWriteTool) UserFacingName() string                   { return "write" }
func (t *FileWriteTool) Description() string                      { return "Create or overwrite a file with new content." }
func (t *FileWriteTool) IsReadOnly(_ json.RawMessage) bool        { return false }
func (t *FileWriteTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (t *FileWriteTool) MaxResultSizeChars() int                  { return 0 }
func (t *FileWriteTool) IsEnabled(_ *tool.UseContext) bool        { return true }
func (t *FileWriteTool) IsDestructive(_ json.RawMessage) bool     { return true }
func (t *FileWriteTool) ShouldDefer() bool                        { return true }
func (t *FileWriteTool) GetPath(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return ""
	}
	return in.FilePath
}
func (t *FileWriteTool) GetActivityDescription(input json.RawMessage) string {
	if p := t.GetPath(input); p != "" {
		return "Writing " + p
	}
	return "Writing file"
}
func (t *FileWriteTool) GetToolUseSummary(input json.RawMessage) string {
	return t.GetActivityDescription(input)
}
func (t *FileWriteTool) ToAutoClassifierInput(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return ""
	}
	return in.FilePath
}
func (t *FileWriteTool) InputsEquivalent(a, b json.RawMessage) bool {
	var ia, ib Input
	if json.Unmarshal(a, &ia) != nil || json.Unmarshal(b, &ib) != nil {
		return false
	}
	return ia.FilePath == ib.FilePath && ia.Content == ib.Content
}

func (t *FileWriteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"file_path":{"type":"string","description":"Absolute path of the file to write."},
			"content":{"type":"string","description":"Full content to write to the file."}
		},
		"required":["file_path","content"]
	}`)
}

func (t *FileWriteTool) OutputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"result": {"type": "string", "description": "Success message with bytes written."},
			"file_path": {"type": "string", "description": "Absolute path of the file that was written."}
		}
	}`)
}

func (t *FileWriteTool) Prompt(_ *tool.UseContext) string {
	return `Writes a file to the local filesystem.

Usage:
- This tool will overwrite the existing file if there is one at the provided path.
- If this is an existing file, you MUST use the Read tool first to read the file's contents. This tool will fail if you did not read the file first.
- Prefer the Edit tool for modifying existing files — it only sends the diff. Only use this tool to create new files or for complete rewrites.
- NEVER create documentation files (*.md) or README files unless explicitly requested by the User.
- Only use emojis if the user explicitly requests it. Avoid writing emojis to files unless asked.`
}

func (t *FileWriteTool) ValidateInput(_ context.Context, input json.RawMessage) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.FilePath == "" {
		return fmt.Errorf("file_path must not be empty")
	}
	if util.IsUNCPath(in.FilePath) {
		return fmt.Errorf("UNC paths are not allowed")
	}
	if util.IsBlockedDevicePath(in.FilePath) {
		return fmt.Errorf("cannot write to device file %q", in.FilePath)
	}
	return nil
}

func (t *FileWriteTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.FilePath == "" {
		return fmt.Errorf("file_path must not be empty")
	}
	return nil
}

func (t *FileWriteTool) Call(_ context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)

		path := in.FilePath
		if !filepath.IsAbs(path) {
			path = filepath.Join(uctx.WorkDir, path)
		}

		// If file exists, require that it was previously read.
		if _, statErr := os.Stat(path); statErr == nil {
			if _, wasCached := fileread.GetCached(path); !wasCached {
				ch <- &engine.ContentBlock{
					Type:    engine.ContentTypeText,
					Text:    fmt.Sprintf("You must read the file before overwriting it. Use the Read tool on %s first.", path),
					IsError: true,
				}
				return
			}
		}

		if err := util.WriteTextContent(path, in.Content); err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
			return
		}

		// Invalidate read cache so subsequent reads pick up the new content.
		fileread.InvalidateCache(path)

		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: fmt.Sprintf("Successfully wrote %d bytes to %s", len(in.Content), path),
		}
	}()
	return ch, nil
}
