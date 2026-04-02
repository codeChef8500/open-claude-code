package glob

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

type Input struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

type GlobTool struct{ tool.BaseTool }

func New() *GlobTool { return &GlobTool{} }

func (t *GlobTool) Name() string                             { return "Glob" }
func (t *GlobTool) UserFacingName() string                   { return "glob" }
func (t *GlobTool) Description() string                      { return "Find files matching a glob pattern." }
func (t *GlobTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (t *GlobTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *GlobTool) MaxResultSizeChars() int                  { return 50_000 }
func (t *GlobTool) IsEnabled(_ *tool.UseContext) bool        { return true }
func (t *GlobTool) IsSearchOrRead(_ json.RawMessage) engine.SearchOrReadInfo {
	return engine.SearchOrReadInfo{IsSearch: true, IsList: true}
}
func (t *GlobTool) GetActivityDescription(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return "Searching files"
	}
	return "Searching: " + in.Pattern
}
func (t *GlobTool) GetToolUseSummary(input json.RawMessage) string {
	return t.GetActivityDescription(input)
}
func (t *GlobTool) ToAutoClassifierInput(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return ""
	}
	return in.Pattern
}

func (t *GlobTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"pattern":{"type":"string","description":"Glob pattern (supports **). E.g. '**/*.go'."},
			"path":{"type":"string","description":"Root directory to search. Defaults to cwd."}
		},
		"required":["pattern"]
	}`)
}

func (t *GlobTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *GlobTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Pattern == "" {
		return fmt.Errorf("pattern must not be empty")
	}
	return nil
}

func (t *GlobTool) Call(_ context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)

		root := in.Path
		if root == "" {
			root = uctx.WorkDir
		} else if !filepath.IsAbs(root) {
			root = filepath.Join(uctx.WorkDir, root)
		}
		root = filepath.ToSlash(root)

		pattern := in.Pattern
		if !strings.Contains(pattern, "/") {
			pattern = "**/" + pattern
		}

		fsys := os.DirFS(root)
		matches, err := doublestar.Glob(fsys, pattern)
		if err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
			return
		}

		if len(matches) == 0 {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "No files found."}
			return
		}

		out := strings.Join(matches, "\n")
		if len(out) > 50_000 {
			out = out[:50_000] + "\n[... truncated ...]"
		}
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: out}
	}()
	return ch, nil
}
