package command

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// Shell execution engine for prompt commands.
// Aligned with claude-code-main utils/promptShellExecution.ts.
//
// Two syntaxes are supported in prompt text:
//   - Block:  ```! command ```
//   - Inline: !`command`
//
// Commands are executed in parallel and their output replaces the pattern.
// ──────────────────────────────────────────────────────────────────────────────

// shellBlockPattern matches ```! command ``` blocks.
var shellBlockPattern = regexp.MustCompile("(?s)```!\\s*\n?(.*?)\n?```")

// shellInlinePattern matches !`command` inline syntax.
// Requires start-of-line or whitespace before the !` to avoid false matches.
var shellInlinePattern = regexp.MustCompile("(?m)(?:^|\\s)!`([^`]+)`")

// shellMatch represents a single shell command match in prompt text.
type shellMatch struct {
	fullMatch string // the full matched pattern (e.g., !`git status`)
	command   string // the extracted command
}

// ExecuteShellCommandsInPrompt parses prompt text and executes any embedded
// shell commands, replacing the patterns with command output.
// This is the Go equivalent of claude-code-main's executeShellCommandsInPrompt().
func ExecuteShellCommandsInPrompt(
	ctx context.Context,
	text string,
	workDir string,
	shellSvc ShellService,
	timeoutSec int,
) (string, error) {
	if shellSvc == nil {
		return text, nil
	}
	if timeoutSec <= 0 {
		timeoutSec = 30
	}

	matches := findShellMatches(text)
	if len(matches) == 0 {
		return text, nil
	}

	// Execute all shell commands in parallel.
	type result struct {
		match  shellMatch
		output string
		err    error
	}

	results := make([]result, len(matches))
	var wg sync.WaitGroup

	for i, m := range matches {
		wg.Add(1)
		go func(idx int, sm shellMatch) {
			defer wg.Done()
			out, err := shellSvc.ExecuteWithTimeout(ctx, workDir, sm.command, timeoutSec)
			results[idx] = result{match: sm, output: out, err: err}
		}(i, m)
	}
	wg.Wait()

	// Replace patterns with outputs.
	output := text
	for _, r := range results {
		var replacement string
		if r.err != nil {
			replacement = fmt.Sprintf("[Error: %s]", r.err.Error())
		} else {
			replacement = strings.TrimSpace(r.output)
		}
		output = strings.Replace(output, r.match.fullMatch, replacement, 1)
	}

	return output, nil
}

// findShellMatches extracts all shell command patterns from text.
func findShellMatches(text string) []shellMatch {
	var matches []shellMatch

	// Block matches: ```! command ```
	for _, m := range shellBlockPattern.FindAllStringSubmatch(text, -1) {
		cmd := strings.TrimSpace(m[1])
		if cmd != "" {
			matches = append(matches, shellMatch{fullMatch: m[0], command: cmd})
		}
	}

	// Inline matches: !`command` — only scan if !` is present (optimization).
	if strings.Contains(text, "!`") {
		for _, m := range shellInlinePattern.FindAllStringSubmatch(text, -1) {
			cmd := strings.TrimSpace(m[1])
			if cmd != "" {
				// The full match may include leading whitespace; use the
				// portion starting from !` for replacement.
				full := m[0]
				idx := strings.Index(full, "!`")
				if idx >= 0 {
					full = full[idx:]
				}
				matches = append(matches, shellMatch{fullMatch: full, command: cmd})
			}
		}
	}

	return matches
}

// ──────────────────────────────────────────────────────────────────────────────
// Default ShellService implementation using os/exec.
// ──────────────────────────────────────────────────────────────────────────────

// DefaultShellService implements ShellService using os/exec.
type DefaultShellService struct{}

func (s *DefaultShellService) Execute(ctx context.Context, dir string, command string) (string, error) {
	return s.ExecuteWithTimeout(ctx, dir, command, 30)
}

func (s *DefaultShellService) ExecuteWithTimeout(ctx context.Context, dir string, command string, timeoutSec int) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(out), fmt.Errorf("command timed out after %ds: %s", timeoutSec, command)
	}
	if err != nil {
		// Return output even on error (like stderr), but also the error.
		return string(out), fmt.Errorf("command failed: %w\nOutput: %s", err, string(out))
	}
	return string(out), nil
}

// ──────────────────────────────────────────────────────────────────────────────
// AllowedTools — temporary permission elevation for prompt commands.
// Aligned with claude-code-main's allowedTools in PromptCommand.
// ──────────────────────────────────────────────────────────────────────────────

// AllowedToolsOverride represents a temporary tool permission override
// that prompt commands can declare (e.g., /commit allows Bash(git add:*)).
type AllowedToolsOverride struct {
	// Tools is the list of allowed tool patterns.
	// Format: "ToolName(pattern)" e.g. "Bash(git add:*)", "Read", "Grep"
	Tools []string
}

// MatchesTool checks if a tool invocation is allowed by this override.
func (o *AllowedToolsOverride) MatchesTool(toolName string, input string) bool {
	if o == nil || len(o.Tools) == 0 {
		return false
	}
	for _, pattern := range o.Tools {
		if matchesToolPattern(pattern, toolName, input) {
			return true
		}
	}
	return false
}

// matchesToolPattern checks if a tool name and input match an allowed-tool pattern.
// Patterns:
//   - "Read" — matches tool name exactly
//   - "Bash(git add:*)" — matches tool name + input prefix
//   - "Bash(git *)" — matches tool name + input glob
func matchesToolPattern(pattern, toolName, input string) bool {
	// Check for parameterized pattern: ToolName(pattern)
	parenIdx := strings.Index(pattern, "(")
	if parenIdx < 0 {
		// Simple name match.
		return strings.EqualFold(strings.TrimSpace(pattern), toolName)
	}

	// Extract tool name and input pattern.
	pToolName := strings.TrimSpace(pattern[:parenIdx])
	if !strings.EqualFold(pToolName, toolName) {
		return false
	}

	// Extract the inner pattern (remove trailing ')').
	inner := pattern[parenIdx+1:]
	inner = strings.TrimSuffix(inner, ")")
	inner = strings.TrimSpace(inner)

	if inner == "*" {
		return true
	}

	// Check prefix match with wildcard: "git add:*" matches "git add --all"
	if strings.HasSuffix(inner, ":*") {
		prefix := strings.TrimSuffix(inner, ":*")
		return strings.HasPrefix(input, prefix)
	}
	if strings.HasSuffix(inner, " *") {
		prefix := strings.TrimSuffix(inner, " *")
		return strings.HasPrefix(input, prefix)
	}

	// Exact match.
	return input == inner
}

// ParseAllowedToolsFromFrontmatter parses an "allowed-tools" frontmatter value
// into an AllowedToolsOverride. The value is a comma-separated list of tool patterns.
// Aligned with claude-code-main's parseSlashCommandToolsFromFrontmatter().
func ParseAllowedToolsFromFrontmatter(value string) *AllowedToolsOverride {
	if value == "" {
		return nil
	}
	var tools []string
	for _, t := range strings.Split(value, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tools = append(tools, t)
		}
	}
	if len(tools) == 0 {
		return nil
	}
	return &AllowedToolsOverride{Tools: tools}
}
