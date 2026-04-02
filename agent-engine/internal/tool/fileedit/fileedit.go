package fileedit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
	"github.com/wall-ai/agent-engine/internal/util"
)

type Input struct {
	FilePath  string `json:"file_path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

type FileEditTool struct{ tool.BaseTool }

func New() *FileEditTool { return &FileEditTool{} }

func (t *FileEditTool) Name() string                      { return "Edit" }
func (t *FileEditTool) UserFacingName() string            { return "edit" }
func (t *FileEditTool) Description() string               { return "Replace an exact string in a file." }
func (t *FileEditTool) IsReadOnly() bool                  { return false }
func (t *FileEditTool) IsConcurrencySafe() bool           { return false }
func (t *FileEditTool) MaxResultSizeChars() int           { return 0 }
func (t *FileEditTool) IsEnabled(_ *tool.UseContext) bool { return true }
func (t *FileEditTool) IsDestructive() bool               { return true }
func (t *FileEditTool) ShouldDefer() bool                 { return true }
func (t *FileEditTool) GetPath(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return ""
	}
	return in.FilePath
}
func (t *FileEditTool) GetActivityDescription(input json.RawMessage) string {
	if p := t.GetPath(input); p != "" {
		return "Editing " + p
	}
	return "Editing file"
}

func (t *FileEditTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"file_path":{"type":"string","description":"Absolute path to the file to edit."},
			"old_string":{"type":"string","description":"The exact text to replace (must be unique in the file)."},
			"new_string":{"type":"string","description":"The replacement text."}
		},
		"required":["file_path","old_string","new_string"]
	}`)
}

func (t *FileEditTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *FileEditTool) CheckPermissions(_ context.Context, input json.RawMessage, uctx *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.FilePath == "" {
		return fmt.Errorf("file_path must not be empty")
	}
	return nil
}

func (t *FileEditTool) Call(_ context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
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

		// Record modification time before reading.
		statBefore, err := os.Stat(path)
		if err != nil {
			ch <- errBlock("stat file: " + err.Error())
			return
		}
		modBefore := statBefore.ModTime()

		content, err := os.ReadFile(path)
		if err != nil {
			ch <- errBlock("read file: " + err.Error())
			return
		}
		text := string(content)

		// Detect concurrent modification.
		time.Sleep(1 * time.Millisecond)
		statAfter, err := os.Stat(path)
		if err == nil && statAfter.ModTime() != modBefore {
			ch <- errBlock("file was modified concurrently; please re-read before editing")
			return
		}

		// Uniqueness check.
		count := strings.Count(text, in.OldString)
		if count == 0 {
			ch <- errBlock(fmt.Sprintf("old_string not found in file %q", path))
			return
		}
		if count > 1 {
			ch <- errBlock(fmt.Sprintf("old_string appears %d times in file; must be unique", count))
			return
		}

		// Preserve line endings.
		newText := strings.Replace(text, in.OldString, in.NewString, 1)

		if err := util.WriteTextContent(path, newText); err != nil {
			ch <- errBlock("write file: " + err.Error())
			return
		}

		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: fmt.Sprintf("Successfully edited %s", path),
		}
	}()
	return ch, nil
}

func errBlock(msg string) *engine.ContentBlock {
	return &engine.ContentBlock{Type: engine.ContentTypeText, Text: msg, IsError: true}
}
