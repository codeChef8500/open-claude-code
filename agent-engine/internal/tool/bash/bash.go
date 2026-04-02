package bash

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
	"github.com/wall-ai/agent-engine/internal/util"
)

const (
	defaultTimeout = 120_000 // 2 minutes in ms
	maxOutputChars = 100_000
)

// Input is the JSON input schema for BashTool.
type Input struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
	// Description shown to user before execution.
	Description string `json:"description,omitempty"`
	// Restart the shell session.
	Restart bool `json:"restart,omitempty"`
}

// BashTool executes shell commands.
type BashTool struct{ tool.BaseTool }

func New() *BashTool { return &BashTool{} }

func (t *BashTool) Name() string                             { return "Bash" }
func (t *BashTool) UserFacingName() string                   { return "bash" }
func (t *BashTool) Description() string                      { return "Execute a shell command and return its output." }
func (t *BashTool) IsReadOnly(_ json.RawMessage) bool        { return false }
func (t *BashTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (t *BashTool) MaxResultSizeChars() int                  { return maxOutputChars }
func (t *BashTool) IsEnabled(uctx *tool.UseContext) bool     { return true }
func (t *BashTool) IsDestructive(_ json.RawMessage) bool     { return true }
func (t *BashTool) ShouldDefer() bool                        { return true }
func (t *BashTool) InterruptBehavior() engine.InterruptBehavior {
	return engine.InterruptBehaviorReturn
}
func (t *BashTool) GetActivityDescription(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil || in.Command == "" {
		return "Running bash command"
	}
	cmd := in.Command
	if len(cmd) > 60 {
		cmd = cmd[:60] + "…"
	}
	return "Running: " + cmd
}
func (t *BashTool) GetToolUseSummary(input json.RawMessage) string {
	return t.GetActivityDescription(input)
}
func (t *BashTool) Aliases() []string { return []string{"bash", "shell"} }
func (t *BashTool) ToAutoClassifierInput(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return ""
	}
	return in.Command
}
func (t *BashTool) IsSearchOrRead(input json.RawMessage) engine.SearchOrReadInfo {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return engine.SearchOrReadInfo{}
	}
	return classifyBashCommand(in.Command)
}

func (t *BashTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"command":{"type":"string","description":"The shell command to execute."},
			"timeout":{"type":"integer","description":"Timeout in milliseconds (default 120000)."},
			"description":{"type":"string","description":"Brief description shown to the user."},
			"restart":{"type":"boolean","description":"Restart the shell session."}
		},
		"required":["command"]
	}`)
}

func (t *BashTool) Prompt(uctx *tool.UseContext) string {
	return `## BashTool
Run shell commands. Commands time out after 2 minutes by default.
- Avoid interactive commands that wait for stdin.
- Use 'timeout' parameter if you need longer execution.
- Prefer non-destructive commands; ask before deleting files.`
}

func (t *BashTool) CheckPermissions(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Command == "" && !in.Restart {
		return fmt.Errorf("command must not be empty")
	}
	// Shell AST safety check — detects dangerous patterns via syntax tree.
	if in.Command != "" {
		if err := checkShellAST(in.Command); err != nil {
			return fmt.Errorf("shell safety check: %w", err)
		}
	}
	return nil
}

func (t *BashTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 4)
	go func() {
		defer close(ch)

		if in.Restart {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "Shell session restarted."}
			return
		}

		timeout := in.Timeout
		if timeout <= 0 {
			timeout = defaultTimeout
		}

		result, err := util.Exec(ctx, in.Command, &util.ExecOptions{
			CWD:       uctx.WorkDir,
			TimeoutMs: timeout,
		})
		if err != nil {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    "Error: " + err.Error(),
				IsError: true,
			}
			return
		}

		output := buildOutput(result)
		ch <- &engine.ContentBlock{
			Type:    engine.ContentTypeText,
			Text:    output,
			IsError: result.ExitCode != 0,
		}
	}()
	return ch, nil
}

// classifyBashCommand checks if a command is a search or read operation.
func classifyBashCommand(command string) engine.SearchOrReadInfo {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return engine.SearchOrReadInfo{}
	}
	// Extract the first token (the actual binary).
	first := strings.Fields(cmd)[0]
	// Strip path prefixes.
	if idx := strings.LastIndex(first, "/"); idx >= 0 {
		first = first[idx+1:]
	}
	switch first {
	case "grep", "rg", "ag", "ack", "find", "fd", "fzf", "locate":
		return engine.SearchOrReadInfo{IsSearch: true}
	case "cat", "head", "tail", "less", "more", "wc", "file", "stat", "ls", "dir", "tree", "du", "df":
		return engine.SearchOrReadInfo{IsRead: true}
	}
	if ok, _ := IsReadOnlyCommand(cmd); ok {
		return engine.SearchOrReadInfo{IsRead: true}
	}
	return engine.SearchOrReadInfo{}
}

func buildOutput(r *util.ExecResult) string {
	out := r.Stdout
	if r.Stderr != "" {
		if out != "" {
			out += "\n"
		}
		out += r.Stderr
	}
	if len(out) > maxOutputChars {
		out = out[:maxOutputChars] + "\n[... output truncated ...]"
	}
	if r.ExitCode != 0 {
		out += fmt.Sprintf("\n\nExit code: %d", r.ExitCode)
	}
	if out == "" {
		out = "(no output)"
	}
	return out
}
