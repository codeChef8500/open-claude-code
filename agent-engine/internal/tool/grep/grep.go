package grep

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
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

type GrepTool struct{ tool.BaseTool }

func New() *GrepTool { return &GrepTool{} }

func (t *GrepTool) Name() string                      { return "Grep" }
func (t *GrepTool) UserFacingName() string            { return "grep" }
func (t *GrepTool) Description() string               { return "Search files for a pattern using ripgrep." }
func (t *GrepTool) IsReadOnly(_ json.RawMessage) bool                  { return true }
func (t *GrepTool) IsConcurrencySafe(_ json.RawMessage) bool           { return true }
func (t *GrepTool) MaxResultSizeChars() int           { return maxOutputChars }
func (t *GrepTool) IsEnabled(_ *tool.UseContext) bool { return true }
func (t *GrepTool) IsSearchOrRead(_ json.RawMessage) engine.SearchOrReadInfo { return engine.SearchOrReadInfo{IsSearch: true} }

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

		// Use ripgrep when available; fall back to pure-Go walk+regexp otherwise.
		if _, lookErr := exec.LookPath("rg"); lookErr == nil {
			result, err := util.Exec(ctx, cmd.String(), &util.ExecOptions{CWD: uctx.WorkDir})
			if err != nil {
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
			return
		}

		// Pure-Go fallback.
		out, err := goGrep(ctx, in.Pattern, searchPath, in.Include)
		if err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "Error: " + err.Error(), IsError: true}
			return
		}
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

// goGrep is a pure-Go implementation of grep using filepath.WalkDir + regexp.
func goGrep(ctx context.Context, pattern, root, include string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid pattern: %w", err)
	}

	var sb strings.Builder
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil || d.IsDir() {
			if d != nil && d.IsDir() && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if include != "" {
			matched, _ := doublestar.Match(include, d.Name())
			if !matched {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				fmt.Fprintf(&sb, "%s:%d:%s\n", path, lineNum, line)
				if sb.Len() > maxOutputChars {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return "", err
	}
	return sb.String(), nil
}
