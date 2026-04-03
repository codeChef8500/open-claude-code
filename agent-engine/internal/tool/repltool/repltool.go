package repltool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
	"github.com/wall-ai/agent-engine/internal/util"
)

// ────────────────────────────────────────────────────────────────────────────
// REPLTool — executes code in a language-specific REPL (Python, Node, etc.).
// Aligned with claude-code-main's REPL execution pattern.
// ────────────────────────────────────────────────────────────────────────────

const (
	defaultREPLTimeout = 30_000 // 30 seconds
	maxREPLOutput      = 50_000
)

// Input is the JSON input schema for REPLTool.
type Input struct {
	Language string `json:"language"`
	Code     string `json:"code"`
	Timeout  int    `json:"timeout,omitempty"`
}

// REPLTool executes code snippets in a REPL environment.
type REPLTool struct{ tool.BaseTool }

func New() *REPLTool { return &REPLTool{} }

func (t *REPLTool) Name() string           { return "REPL" }
func (t *REPLTool) UserFacingName() string { return "repl" }
func (t *REPLTool) Description() string {
	return "Execute code in a REPL (Python, Node.js, etc.) and return the output."
}
func (t *REPLTool) IsReadOnly(_ json.RawMessage) bool        { return false }
func (t *REPLTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (t *REPLTool) MaxResultSizeChars() int                  { return maxREPLOutput }
func (t *REPLTool) IsEnabled(uctx *tool.UseContext) bool     { return true }
func (t *REPLTool) IsDestructive(_ json.RawMessage) bool     { return true }
func (t *REPLTool) Aliases() []string                        { return []string{"repl", "execute_code"} }
func (t *REPLTool) InterruptBehavior() engine.InterruptBehavior {
	return engine.InterruptBehaviorReturn
}

func (t *REPLTool) GetActivityDescription(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return "Running code"
	}
	lang := in.Language
	if lang == "" {
		lang = "python"
	}
	lines := strings.Count(in.Code, "\n") + 1
	return fmt.Sprintf("Running %s (%d lines)", lang, lines)
}

func (t *REPLTool) GetToolUseSummary(input json.RawMessage) string {
	return t.GetActivityDescription(input)
}

func (t *REPLTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"language":{"type":"string","description":"Programming language: python, node, ruby, etc.","enum":["python","node","ruby","bash"]},
			"code":{"type":"string","description":"The code to execute."},
			"timeout":{"type":"integer","description":"Timeout in milliseconds (default 30000)."}
		},
		"required":["code"]
	}`)
}

func (t *REPLTool) Prompt(uctx *tool.UseContext) string {
	return `## REPL Tool
Execute code snippets in a REPL environment.
- Supported languages: python, node, ruby, bash
- Default language is python
- Code executes in a subprocess; avoid infinite loops
- Use for quick calculations, data exploration, testing snippets`
}

func (t *REPLTool) CheckPermissions(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if strings.TrimSpace(in.Code) == "" {
		return fmt.Errorf("code must not be empty")
	}
	return nil
}

func (t *REPLTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	lang := in.Language
	if lang == "" {
		lang = "python"
	}

	timeout := in.Timeout
	if timeout <= 0 {
		timeout = defaultREPLTimeout
	}

	ch := make(chan *engine.ContentBlock, 4)
	go func() {
		defer close(ch)

		cmd, flag := resolveREPL(lang)
		if cmd == "" {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    fmt.Sprintf("Unsupported language: %s", lang),
				IsError: true,
			}
			return
		}

		// Build the command: pipe code via -c / -e flag.
		shellCmd := fmt.Sprintf("%s %s %s", cmd, flag, shellQuote(in.Code))

		result, err := util.Exec(ctx, shellCmd, &util.ExecOptions{
			CWD:       uctx.WorkDir,
			TimeoutMs: timeout,
		})
		if err != nil {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    "REPL error: " + err.Error(),
				IsError: true,
			}
			return
		}

		output := buildREPLOutput(result, lang)
		ch <- &engine.ContentBlock{
			Type:    engine.ContentTypeText,
			Text:    output,
			IsError: result.ExitCode != 0,
		}
	}()
	return ch, nil
}

// resolveREPL returns the command and code-flag for the given language.
func resolveREPL(lang string) (string, string) {
	switch strings.ToLower(lang) {
	case "python", "python3", "py":
		return "python3", "-c"
	case "node", "nodejs", "javascript", "js":
		return "node", "-e"
	case "ruby", "rb":
		return "ruby", "-e"
	case "bash", "sh":
		return "bash", "-c"
	default:
		return "", ""
	}
}

func buildREPLOutput(r *util.ExecResult, lang string) string {
	out := r.Stdout
	if r.Stderr != "" {
		if out != "" {
			out += "\n--- stderr ---\n"
		}
		out += r.Stderr
	}
	if len(out) > maxREPLOutput {
		out = out[:maxREPLOutput] + "\n[... output truncated ...]"
	}
	if r.ExitCode != 0 {
		out += fmt.Sprintf("\n\nExit code: %d", r.ExitCode)
	}
	if out == "" {
		out = "(no output)"
	}
	return fmt.Sprintf("[%s]\n%s", lang, out)
}

func shellQuote(s string) string {
	// Use single quotes and escape any embedded single quotes.
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
