package prompt

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// shellCmdRe matches inline shell commands in prompt templates: !`command`
var shellCmdRe = regexp.MustCompile("!`([^`]+)`")

// ExpandShellCommands expands inline !`command` references in prompt text
// by executing each command and replacing with the output. This is used by
// prompt commands like /commit and /review that embed live shell output.
//
// Commands are executed in workDir with a 10-second timeout per command.
// Errors produce an inline "[error: ...]" marker instead of failing.
func ExpandShellCommands(ctx context.Context, text, workDir string) string {
	return shellCmdRe.ReplaceAllStringFunc(text, func(match string) string {
		sub := shellCmdRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		cmdStr := sub[1]
		out, err := runShellCommand(ctx, cmdStr, workDir)
		if err != nil {
			return fmt.Sprintf("[error running %q: %v]", cmdStr, err)
		}
		return strings.TrimSpace(out)
	})
}

// runShellCommand executes a single shell command with a timeout.
func runShellCommand(ctx context.Context, cmdStr, workDir string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	// Use shell to handle pipes and redirects.
	cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = workDir

	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("command timed out after 10s")
	}
	if err != nil {
		// Still return output if there was some.
		if len(out) > 0 {
			return string(out), nil
		}
		return "", err
	}
	return string(out), nil
}

// TruncateShellOutput limits shell output to maxLen characters, appending
// a truncation marker if needed.
func TruncateShellOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen] + "\n[... output truncated ...]"
}

// BuildPromptWithShellExpansion processes a prompt command's content,
// expanding shell commands and applying output limits.
func BuildPromptWithShellExpansion(ctx context.Context, content, workDir string, maxOutputPerCmd int) string {
	if maxOutputPerCmd <= 0 {
		maxOutputPerCmd = 8000
	}

	// Expand shell commands.
	expanded := shellCmdRe.ReplaceAllStringFunc(content, func(match string) string {
		sub := shellCmdRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		cmdStr := sub[1]
		out, err := runShellCommand(ctx, cmdStr, workDir)
		if err != nil {
			return fmt.Sprintf("[error running %q: %v]", cmdStr, err)
		}
		return TruncateShellOutput(strings.TrimSpace(out), maxOutputPerCmd)
	})

	return expanded
}
