package skill

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// ShellExecContext provides the environment for shell command execution in prompts.
type ShellExecContext struct {
	// WorkDir is the working directory for shell commands.
	WorkDir string
	// Shell overrides the default shell ("bash" or "powershell"). Empty = auto-detect.
	Shell string
	// Timeout for each shell command. Zero means 30s default.
	Timeout time.Duration
	// Ctx is the parent context for cancellation.
	Ctx context.Context
}

// SubstituteVariables replaces ${VAR_NAME} placeholders in content with values
// from the vars map. Matches claude-code-main's variable substitution:
//
//	${CLAUDE_SKILL_DIR}, ${CLAUDE_SESSION_ID}, ${CLAUDE_PLUGIN_ROOT},
//	${CLAUDE_PLUGIN_DATA}, ${user_config.KEY}, etc.
func SubstituteVariables(content string, vars map[string]string) string {
	if len(vars) == 0 || !strings.Contains(content, "${") {
		return content
	}
	result := content
	for k, v := range vars {
		result = strings.ReplaceAll(result, "${"+k+"}", v)
	}
	return result
}

var (
	// Block pattern: ```! command ``` (multiline)
	blockShellRe = regexp.MustCompile("(?s)```!\\s*\n?(.*?)\n?```")
	// Inline pattern: !`command`
	// Preceded by start-of-line or whitespace to avoid false matches.
	inlineShellRe = regexp.MustCompile("(?m)(?:^|\\s)!`([^`]+)`")
)

// ExecuteShellCommandsInPrompt parses prompt text and executes embedded shell
// commands, replacing them with their stdout output. Supports two syntaxes:
//
//	Code blocks: ```! command ```
//	Inline:      !`command`
//
// Matches claude-code-main's executeShellCommandsInPrompt.
func ExecuteShellCommandsInPrompt(text string, ectx ShellExecContext) (string, error) {
	if !strings.Contains(text, "!`") && !strings.Contains(text, "```!") {
		return text, nil
	}

	ctx := ectx.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := ectx.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	var firstErr error
	recordErr := func(err error) {
		if firstErr == nil {
			firstErr = err
		}
	}

	// Replace block patterns
	result := blockShellRe.ReplaceAllStringFunc(text, func(match string) string {
		sub := blockShellRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		cmd := strings.TrimSpace(sub[1])
		if cmd == "" {
			return match
		}
		out, err := runShellCommand(ctx, cmd, ectx.WorkDir, ectx.Shell, timeout)
		if err != nil {
			recordErr(fmt.Errorf("block shell command %q: %w", cmd, err))
			return fmt.Sprintf("[shell error: %s]", err)
		}
		return out
	})

	// Replace inline patterns
	result = inlineShellRe.ReplaceAllStringFunc(result, func(match string) string {
		sub := inlineShellRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		cmd := strings.TrimSpace(sub[1])
		if cmd == "" {
			return match
		}
		// Preserve leading whitespace from the match
		prefix := ""
		if len(match) > 0 && (match[0] == ' ' || match[0] == '\t' || match[0] == '\n') {
			prefix = string(match[0])
		}
		out, err := runShellCommand(ctx, cmd, ectx.WorkDir, ectx.Shell, timeout)
		if err != nil {
			recordErr(fmt.Errorf("inline shell command %q: %w", cmd, err))
			return prefix + fmt.Sprintf("[shell error: %s]", err)
		}
		return prefix + strings.TrimSpace(out)
	})

	return result, firstErr
}

// runShellCommand executes a single command in the specified shell.
func runShellCommand(ctx context.Context, command, workDir, shell string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	shellBin, shellArg := resolveShell(shell)
	cmd := exec.CommandContext(ctx, shellBin, shellArg, command)
	if workDir != "" {
		cmd.Dir = workDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return "", fmt.Errorf("%w: %s", err, stderrStr)
		}
		return "", err
	}

	return stdout.String(), nil
}

// resolveShell returns the shell binary and its command flag.
func resolveShell(shell string) (string, string) {
	switch strings.ToLower(shell) {
	case "powershell", "pwsh":
		return "powershell", "-Command"
	case "bash":
		return "bash", "-c"
	case "sh":
		return "sh", "-c"
	default:
		// Auto-detect based on OS.
		if runtime.GOOS == "windows" {
			return "powershell", "-Command"
		}
		return "bash", "-c"
	}
}
