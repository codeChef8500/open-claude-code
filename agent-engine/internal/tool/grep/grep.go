package grep

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
	"github.com/wall-ai/agent-engine/internal/util"
)

const maxOutputChars = 100_000

type Input struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
	Include string `json:"include,omitempty"`
}

type GrepTool struct{}

func New() *GrepTool { return &GrepTool{} }

func (t *GrepTool) Name() string            { return "Grep" }
func (t *GrepTool) UserFacingName() string  { return "grep" }
func (t *GrepTool) Description() string     { return "Search files for a pattern using ripgrep." }
func (t *GrepTool) IsReadOnly() bool        { return true }
func (t *GrepTool) IsConcurrencySafe() bool { return true }
func (t *GrepTool) MaxResultSizeChars() int { return maxOutputChars }
func (t *GrepTool) IsEnabled(_ *tool.UseContext) bool { return true }

func (t *GrepTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"pattern":{"type":"string","description":"The regex pattern to search for."},
			"path":{"type":"string","description":"Directory or file to search. Defaults to cwd."},
			"include":{"type":"string","description":"Glob pattern to filter files (e.g. '*.go')."}
		},
		"required":["pattern"]
	}`)
}

func (t *GrepTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *GrepTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Pattern == "" {
		return fmt.Errorf("pattern must not be empty")
	}
	return nil
}

func (t *GrepTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)

		searchPath := in.Path
		if searchPath == "" {
			searchPath = uctx.WorkDir
		} else if !filepath.IsAbs(searchPath) {
			searchPath = filepath.Join(uctx.WorkDir, searchPath)
		}

		var cmd strings.Builder
		cmd.WriteString("rg --line-number --no-heading --color=never ")
		if in.Include != "" {
			cmd.WriteString(fmt.Sprintf("--glob %s ", util.ShellQuote(in.Include)))
		}
		cmd.WriteString(util.ShellQuote(in.Pattern))
		cmd.WriteString(" ")
		cmd.WriteString(util.ShellQuote(searchPath))

		result, err := util.Exec(ctx, cmd.String(), &util.ExecOptions{CWD: uctx.WorkDir})
		if err != nil {
			// rg exits 1 when no matches found — treat as empty result.
			if result != nil && result.ExitCode == 1 {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "No matches found."}
				return
			}
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "Error: " + err.Error(), IsError: true}
			return
		}

		out := result.Stdout
		if len(out) > maxOutputChars {
			out = out[:maxOutputChars] + "\n[... output truncated ...]"
		}
		if out == "" {
			out = "No matches found."
		}
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: out}
	}()
	return ch, nil
}
