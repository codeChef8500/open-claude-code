package bash

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
	"github.com/wall-ai/agent-engine/internal/tool/fileread"
	"github.com/wall-ai/agent-engine/internal/util"
)

const (
	defaultTimeout = 120_000 // 2 minutes in ms
	maxOutputChars = 30_000  // tool result persistence threshold, matches claude-code

	// ProgressThresholdMs is the delay before progress output is shown (2s).
	ProgressThresholdMs = 2000
)

// Output is the structured output of a Bash call.
type Output struct {
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	ExitCode    int    `json:"exitCode"`
	Interrupted bool   `json:"interrupted"`
}

// SedEditInfo describes a parsed sed in-place edit command.
type SedEditInfo struct {
	FilePath    string
	Pattern     string
	Replacement string
	Flags       string
}

// Input is the JSON input schema for BashTool.
type Input struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
	// Description shown to user before execution.
	Description     string `json:"description,omitempty"`
	Restart         bool   `json:"restart,omitempty"`
	RunInBackground bool   `json:"run_in_background,omitempty"`
}

// BashTool executes shell commands.
type BashTool struct{ tool.BaseTool }

func New() *BashTool { return &BashTool{} }

func (t *BashTool) Name() string           { return "Bash" }
func (t *BashTool) UserFacingName() string { return "bash" }
func (t *BashTool) Description() string    { return "Execute a shell command and return its output." }
func (t *BashTool) IsReadOnly(input json.RawMessage) bool {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil || in.Command == "" {
		return false
	}
	ok, _ := IsReadOnlyCommand(in.Command)
	return ok
}
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
			"command":{"type":"string","description":"The shell command to execute. Can be a multi-line script."},
			"timeout":{"type":"integer","description":"Timeout in milliseconds (default 120000). Max 600000."},
			"description":{"type":"string","description":"Brief human-readable description of what this command does, shown to the user for permission."},
			"restart":{"type":"boolean","description":"Restart the shell session before executing."},
			"run_in_background":{"type":"boolean","description":"If true, run the command in the background and return immediately with the process ID."}
		},
		"required":["command"]
	}`)
}

func (t *BashTool) OutputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"stdout":{"type":"string","description":"The standard output of the command."},
			"stderr":{"type":"string","description":"The standard error output of the command."},
			"exitCode":{"type":"integer","description":"The exit code of the command."},
			"interrupted":{"type":"boolean","description":"Whether the command was interrupted."}
		}
	}`)
}

func (t *BashTool) Prompt(uctx *tool.UseContext) string {
	return `Executes a shell command on the system. Use this for system operations, running builds, installing packages, and other command-line tasks.

Usage:
- Commands time out after 120 seconds by default. Use the timeout parameter (in ms) for longer operations (max 600s).
- ALWAYS use the Grep tool for search tasks instead of invoking grep or rg as a Bash command.
- Prefer the Edit tool for modifying existing files instead of sed or awk.
- Avoid interactive commands that wait for stdin. If you must, use timeout or yes to auto-answer.
- Prefer non-destructive commands — ask the user before deleting files or modifying the system.
- For long-running tasks (e.g., dev servers), set run_in_background to true.
- Use set -e at the start of multi-command scripts to fail fast on any error.
- Limit output to only the information needed to avoid context bloat. Use head, tail, or grep to filter.
- If a command produces very long output, pipe it through head -n 100 or similar.

Background execution:
- Use run_in_background: true for dev servers, watch processes, and long-running builds.
- You will get back the process ID. You cannot read the background process output.
- To stop a background process, use kill <pid>.

Git operations:
- NEVER skip hooks (--no-verify, --no-gpg-sign, etc.) unless the user explicitly requests it.
- Use the gh command via Bash for GitHub-related tasks including working with issues, checks, and releases.
- Always provide a meaningful commit message that describes the changes, not just "fix" or "update".
- When creating commits, use git diff --staged to review changes before committing.
- Prefer small, focused commits over large monolithic ones.`
}

func (t *BashTool) ValidateInput(_ context.Context, input json.RawMessage) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Command == "" && !in.Restart {
		return fmt.Errorf("command must not be empty")
	}
	if in.Command != "" && util.IsUNCPath(in.Command) {
		return fmt.Errorf("commands containing UNC paths are not allowed")
	}
	if in.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative")
	}
	return nil
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

		// Background execution.
		if in.RunInBackground {
			cmd := exec.CommandContext(ctx, "bash", "-c", in.Command)
			cmd.Dir = uctx.WorkDir
			if err := cmd.Start(); err != nil {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "Error starting background process: " + err.Error(), IsError: true}
				return
			}
			pid := cmd.Process.Pid
			// Detach: don't wait for the process.
			go func() { _ = cmd.Wait() }()
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: fmt.Sprintf("Started background process with PID %d.", pid)}
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

		// Track sed -i edits: invalidate file cache for modified files.
		trackSedEdits(in.Command, uctx)

		// Track git operations that modify working tree.
		trackGitOps(in.Command, uctx)

		output := buildOutput(result)
		ch <- &engine.ContentBlock{
			Type:    engine.ContentTypeText,
			Text:    output,
			IsError: result.ExitCode != 0,
		}
	}()
	return ch, nil
}

// sedInPlaceRe matches sed -i commands and captures the file path.
var sedInPlaceRe = regexp.MustCompile(`sed\s+(?:-[Ern]*\s+)*-i(?:['"]\S*['"]|\S*)\s+(?:-[Ern]*\s+)*(?:'[^']*'|"[^"]*"|\S+)\s+([^\s;|&]+)`)

// parseSedSubstitution parses a sed substitution expression: s/pattern/replacement/flags.
// Returns (pattern, replacement, flags, ok).
func parseSedSubstitution(expr string) (string, string, string, bool) {
	if len(expr) < 4 || expr[0] != 's' {
		return "", "", "", false
	}
	delim := expr[1]
	rest := expr[2:]

	// Find pattern (up to unescaped delimiter).
	pattern, after, ok := splitAtUnescaped(rest, delim)
	if !ok {
		return "", "", "", false
	}
	// Find replacement (up to unescaped delimiter).
	replacement, flags, ok := splitAtUnescaped(after, delim)
	if !ok {
		return "", "", "", false
	}
	// Validate flags.
	for _, c := range flags {
		switch c {
		case 'g', 'p', 'i', 'm', 'I', 'M':
		default:
			if c >= '1' && c <= '9' {
				continue
			}
			return "", "", "", false
		}
	}
	return pattern, replacement, flags, true
}

// splitAtUnescaped splits s at the first unescaped occurrence of delim.
func splitAtUnescaped(s string, delim byte) (before, after string, ok bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++ // skip escaped char
			continue
		}
		if s[i] == delim {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}

// ParseSedEditCommand attempts to parse a simple sed -i substitution command.
// Returns nil if the command is not a recognizable sed in-place edit.
func ParseSedEditCommand(command string) *SedEditInfo {
	trimmed := strings.TrimSpace(command)
	if !strings.HasPrefix(trimmed, "sed ") {
		return nil
	}

	fields := strings.Fields(trimmed)
	var (
		hasInPlace bool
		extExpr    string
		filePath   string
	)

	i := 1 // skip "sed"
	for i < len(fields) {
		arg := fields[i]
		switch {
		case arg == "-i" || arg == "--in-place":
			hasInPlace = true
			i++
			// macOS: -i '' or -i .bak
			if i < len(fields) {
				next := fields[i]
				if next == "''" || next == `""` || strings.HasPrefix(next, ".") {
					i++
				}
			}
		case strings.HasPrefix(arg, "-i"):
			hasInPlace = true
			i++
		case arg == "-E" || arg == "-r" || arg == "--regexp-extended":
			i++
		case arg == "-e" || arg == "--expression":
			if extExpr != "" {
				return nil // multiple expressions
			}
			if i+1 >= len(fields) {
				return nil
			}
			extExpr = fields[i+1]
			i += 2
		case strings.HasPrefix(arg, "-"):
			return nil // unknown flag
		default:
			if extExpr == "" {
				extExpr = arg
			} else if filePath == "" {
				filePath = arg
			} else {
				return nil // multiple files
			}
			i++
		}
	}

	if !hasInPlace || extExpr == "" || filePath == "" {
		return nil
	}

	// Strip surrounding quotes from expression.
	extExpr = stripQuotes(extExpr)

	// Parse substitution: s/pattern/replacement/flags
	pattern, replacement, flags, ok := parseSedSubstitution(extExpr)
	if !ok {
		return nil
	}

	return &SedEditInfo{
		FilePath:    filePath,
		Pattern:     pattern,
		Replacement: replacement,
		Flags:       flags,
	}
}

func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// ExtractBashCommentLabel extracts a human-readable label from the first line
// of a bash command if it starts with a # comment (not a #! shebang).
// Returns empty string if no comment label is found.
func ExtractBashCommentLabel(command string) string {
	nl := strings.IndexByte(command, '\n')
	firstLine := command
	if nl >= 0 {
		firstLine = command[:nl]
	}
	firstLine = strings.TrimSpace(firstLine)
	if !strings.HasPrefix(firstLine, "#") || strings.HasPrefix(firstLine, "#!") {
		return ""
	}
	// Strip leading # characters and whitespace.
	label := strings.TrimLeft(firstLine, "#")
	label = strings.TrimSpace(label)
	return label
}

// trackSedEdits detects sed -i commands and invalidates the file read cache
// for any files that were modified in-place.
func trackSedEdits(command string, uctx *tool.UseContext) {
	if !strings.Contains(command, "sed") {
		return
	}
	// Try the structured parser first.
	if info := ParseSedEditCommand(command); info != nil {
		fileread.InvalidateCache(info.FilePath)
		return
	}
	// Fallback to regex.
	matches := sedInPlaceRe.FindAllStringSubmatch(command, -1)
	for _, m := range matches {
		if len(m) > 1 {
			fileread.InvalidateCache(m[1])
		}
	}
}

var gitMutatingCmds = map[string]bool{
	"add": true, "commit": true, "push": true, "pull": true,
	"merge": true, "rebase": true, "reset": true, "checkout": true,
	"switch": true, "restore": true, "cherry-pick": true, "revert": true,
	"stash": true, "apply": true, "am": true, "mv": true, "rm": true,
	"clean": true, "bisect": true,
}

// trackGitOps detects git commands that modify the working tree and notifies
// the file history tracker.
func trackGitOps(command string, uctx *tool.UseContext) {
	if !strings.Contains(command, "git") {
		return
	}
	fields := strings.Fields(command)
	for i, f := range fields {
		if f == "git" && i+1 < len(fields) {
			sub := fields[i+1]
			if gitMutatingCmds[sub] && uctx.UpdateFileHistoryState != nil {
				uctx.UpdateFileHistoryState(func(prev *engine.FileHistoryState) *engine.FileHistoryState {
					if prev == nil {
						prev = &engine.FileHistoryState{Files: make(map[string][]engine.FileSnapshot)}
					}
					// Record a sentinel entry indicating git mutation.
					prev.Files["__git_op__"] = append(prev.Files["__git_op__"], engine.FileSnapshot{
						ToolName:  "Bash:git-" + sub,
						ToolUseID: uctx.ToolUseID,
					})
					return prev
				})
			}
			break
		}
	}
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

// MapToolResultToBlockParam formats the bash result for the model,
// including exit code metadata when non-zero.
func (t *BashTool) MapToolResultToBlockParam(content interface{}, toolUseID string) *engine.ContentBlock {
	text, ok := content.(string)
	if !ok {
		return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: ""}
	}
	return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: text}
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
		out += fmt.Sprintf("\nExit code %d", r.ExitCode)
	}
	if out == "" {
		out = "(no output)"
	}
	return out
}
