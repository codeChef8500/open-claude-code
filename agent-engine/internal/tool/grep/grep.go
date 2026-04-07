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
	Pattern       string `json:"pattern"`
	Path          string `json:"path,omitempty"`
	Include       string `json:"include,omitempty"`
	OutputMode    string `json:"output_mode,omitempty"`    // "content" (default), "files_with_matches", "count"
	CaseSensitive *bool  `json:"case_sensitive,omitempty"` // nil = smart case
	ContextLines  int    `json:"context_lines,omitempty"`  // lines of context around matches
	HeadLimit     *int   `json:"head_limit,omitempty"`     // max results; nil = 250, 0 = unlimited
	Offset        int    `json:"offset,omitempty"`         // skip first N results
	Multiline     bool   `json:"multiline,omitempty"`      // enable multiline matching
	Type          string `json:"type,omitempty"`           // ripgrep --type filter (e.g. "go", "py")
}

const defaultHeadLimit = 250

// vcsExcludeDirs are version-control directories automatically excluded from search.
var vcsExcludeDirs = []string{".git", ".svn", ".hg", ".bzr", ".jj", ".sl"}

type GrepTool struct{ tool.BaseTool }

func New() *GrepTool { return &GrepTool{} }

func (t *GrepTool) Name() string                             { return "Grep" }
func (t *GrepTool) UserFacingName() string                   { return "grep" }
func (t *GrepTool) Description() string                      { return "Search files for a pattern using ripgrep." }
func (t *GrepTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (t *GrepTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *GrepTool) MaxResultSizeChars() int                  { return maxOutputChars }
func (t *GrepTool) IsEnabled(_ *tool.UseContext) bool        { return true }
func (t *GrepTool) IsSearchOrRead(_ json.RawMessage) engine.SearchOrReadInfo {
	return engine.SearchOrReadInfo{IsSearch: true}
}

func (t *GrepTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"pattern":{"type":"string","description":"The regex pattern to search for."},
			"path":{"type":"string","description":"Directory or file to search. Defaults to cwd."},
			"include":{"type":"string","description":"Glob pattern to filter files (e.g. '*.go')."},
			"output_mode":{"type":"string","enum":["content","files_with_matches","count"],"description":"Output mode. content: matching lines (default). files_with_matches: only file paths. count: match counts per file."},
			"case_sensitive":{"type":"boolean","description":"Force case-sensitive search. Default: smart case (case-insensitive unless pattern has uppercase)."},
			"context_lines":{"type":"integer","description":"Number of context lines to show around each match."},
			"head_limit":{"type":"integer","description":"Max number of results to return. Default: 250. Set to 0 for unlimited."},
			"offset":{"type":"integer","description":"Skip the first N results (for pagination)."},
			"multiline":{"type":"boolean","description":"Enable multiline matching (. matches newlines)."},
			"type":{"type":"string","description":"Ripgrep file type filter (e.g. 'go', 'py', 'js'). Only used with ripgrep."}
		},
		"required":["pattern"]
	}`)
}

func (t *GrepTool) Prompt(_ *tool.UseContext) string {
	return `A powerful search tool built on ripgrep.

Usage:
- ALWAYS use Grep for search tasks. NEVER invoke grep or rg as a Bash command. The Grep tool has been optimized for correct permissions and access.
- DO NOT USE context_lines for initial searches that may have a large number of results. Use it only when you know it is a very specific, targeted search.
- By default, pattern is treated as a regular expression. Set case_sensitive to control matching.
- Supports full regex syntax (e.g., "log.*Error", "function\s+\w+")
- Filter files with include parameter in glob format (e.g., "*.go", "**/*.tsx")
- Use the Task tool for open-ended searches requiring multiple rounds
- Pattern syntax: Uses ripgrep (not grep) — literal braces need escaping (use interface\{\} to find interface{} in Go code)
- By default, results are capped at 250 matches. Use offset for pagination or set head_limit to 0 for unlimited results.
- If the result is truncated, you must narrow down your search using a more specific pattern or more filters.`
}

func (t *GrepTool) ValidateInput(_ context.Context, input json.RawMessage) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Pattern == "" {
		return fmt.Errorf("pattern must not be empty")
	}
	if in.Path != "" && util.IsUNCPath(in.Path) {
		return fmt.Errorf("UNC paths are not allowed")
	}
	if in.OutputMode != "" && in.OutputMode != "content" && in.OutputMode != "files_with_matches" && in.OutputMode != "count" {
		return fmt.Errorf("output_mode must be \"content\", \"files_with_matches\", or \"count\"")
	}
	// Validate the regex compiles.
	if _, err := regexp.Compile(in.Pattern); err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}
	return nil
}

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

		// Build ripgrep args.
		args := buildRipgrepArgs(in, searchPath)

		// Use ripgrep when available; fall back to pure-Go walk+regexp otherwise.
		if _, lookErr := exec.LookPath("rg"); lookErr == nil {
			cmdStr := "rg " + strings.Join(args, " ")
			result, err := util.Exec(ctx, cmdStr, &util.ExecOptions{CWD: uctx.WorkDir})
			if err != nil {
				if result != nil && result.ExitCode == 1 {
					ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "No matches found."}
					return
				}
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "Error: " + err.Error(), IsError: true}
				return
			}
			out := applyPagination(result.Stdout, in)
			if out == "" {
				out = "No matches found."
			}
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: out}
			return
		}

		// Pure-Go fallback.
		out, err := goGrep(ctx, in, searchPath)
		if err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "Error: " + err.Error(), IsError: true}
			return
		}
		out = applyPagination(out, in)
		if out == "" {
			out = "No matches found."
		}
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: out}
	}()
	return ch, nil
}

// buildRipgrepArgs constructs ripgrep CLI arguments from the Input.
func buildRipgrepArgs(in Input, searchPath string) []string {
	args := []string{"--line-number", "--no-heading", "--color=never"}

	// VCS directory exclusion.
	for _, d := range vcsExcludeDirs {
		args = append(args, "--glob", util.ShellQuote("!"+d))
	}

	// Output mode.
	switch in.OutputMode {
	case "files_with_matches":
		args = append(args, "--files-with-matches")
	case "count":
		args = append(args, "--count")
	}

	// Case sensitivity.
	if in.CaseSensitive != nil {
		if *in.CaseSensitive {
			args = append(args, "--case-sensitive")
		} else {
			args = append(args, "--ignore-case")
		}
	} else {
		args = append(args, "--smart-case")
	}

	// Context lines.
	if in.ContextLines > 0 {
		args = append(args, fmt.Sprintf("--context=%d", in.ContextLines))
	}

	// Multiline.
	if in.Multiline {
		args = append(args, "--multiline", "--multiline-dotall")
	}

	// Include glob.
	if in.Include != "" {
		args = append(args, "--glob", util.ShellQuote(in.Include))
	}

	// Type filter.
	if in.Type != "" {
		args = append(args, "--type", in.Type)
	}

	args = append(args, util.ShellQuote(in.Pattern), util.ShellQuote(searchPath))
	return args
}

// applyPagination applies head_limit and offset to the output string.
func applyPagination(out string, in Input) string {
	limit := defaultHeadLimit
	if in.HeadLimit != nil {
		limit = *in.HeadLimit
	}

	lines := strings.Split(out, "\n")

	// Apply offset.
	if in.Offset > 0 && in.Offset < len(lines) {
		lines = lines[in.Offset:]
	} else if in.Offset >= len(lines) {
		return ""
	}

	// Apply limit (0 = unlimited).
	truncated := false
	if limit > 0 && len(lines) > limit {
		lines = lines[:limit]
		truncated = true
	}

	result := strings.Join(lines, "\n")
	if len(result) > maxOutputChars {
		result = result[:maxOutputChars]
		truncated = true
	}
	if truncated {
		result += fmt.Sprintf("\n[... results truncated. Use offset=%d to see more ...]", in.Offset+limit)
	}
	return result
}

// goGrep is a pure-Go implementation of grep using filepath.WalkDir + regexp.
func goGrep(ctx context.Context, in Input, root string) (string, error) {
	flags := "(?m)"
	if in.CaseSensitive != nil && !*in.CaseSensitive {
		flags += "(?i)"
	}
	if in.Multiline {
		flags += "(?s)"
	}

	re, err := regexp.Compile(flags + in.Pattern)
	if err != nil {
		return "", fmt.Errorf("invalid pattern: %w", err)
	}

	vcsSet := make(map[string]bool, len(vcsExcludeDirs))
	for _, d := range vcsExcludeDirs {
		vcsSet[d] = true
	}

	var sb strings.Builder
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return nil
		}
		if d.IsDir() {
			// Skip hidden and VCS directories.
			if strings.HasPrefix(d.Name(), ".") || vcsSet[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if in.Include != "" {
			matched, _ := doublestar.Match(in.Include, d.Name())
			if !matched {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		f, ferr := os.Open(path)
		if ferr != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		matchCount := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				matchCount++
				switch in.OutputMode {
				case "files_with_matches":
					fmt.Fprintf(&sb, "%s\n", path)
					return nil // one entry per file
				case "count":
					// accumulate; we'll write count after scanning the file
				default:
					fmt.Fprintf(&sb, "%s:%d:%s\n", path, lineNum, line)
				}
				if sb.Len() > maxOutputChars {
					return filepath.SkipAll
				}
			}
		}
		if in.OutputMode == "count" && matchCount > 0 {
			fmt.Fprintf(&sb, "%s:%d\n", path, matchCount)
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return "", err
	}
	return sb.String(), nil
}

// MapToolResultToBlockParam formats the grep result for the model.
func (t *GrepTool) MapToolResultToBlockParam(content interface{}, toolUseID string) *engine.ContentBlock {
	text, ok := content.(string)
	if !ok {
		return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: ""}
	}
	return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: text}
}
