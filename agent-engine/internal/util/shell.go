package util

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const defaultTimeoutMs = 120_000 // 2 minutes

// ExecOptions configures a shell command execution.
type ExecOptions struct {
	CWD       string
	TimeoutMs int
	Env       []string // additional env vars (KEY=VALUE)
	StdinData string
}

// ExecResult holds the output of a completed shell command.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// SafeEnvVars are environment variables always injected for shell safety.
var SafeEnvVars = []string{
	"PAGER=cat",
	"GIT_PAGER=cat",
	"TERM=dumb",
}

// Exec runs command in the system shell with timeout and context cancellation.
// On cancellation it sends SIGTERM first, then SIGKILL after 5 seconds.
func Exec(ctx context.Context, command string, opts *ExecOptions) (*ExecResult, error) {
	if opts == nil {
		opts = &ExecOptions{}
	}

	cwd := opts.CWD
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			cwd = "."
		}
	}

	timeoutMs := opts.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = defaultTimeoutMs
	}

	// Build the environment: inherit current env, then add safe vars and caller extras.
	env := append(os.Environ(), SafeEnvVars...)
	if len(opts.Env) > 0 {
		env = append(env, opts.Env...)
	}

	shellArgs := getShellArgs()

	// Create a child context bound by the timeout.
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	args := append(shellArgs[1:], command)
	cmd := exec.CommandContext(timeoutCtx, shellArgs[0], args...)
	cmd.Dir = cwd
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if opts.StdinData != "" {
		cmd.Stdin = strings.NewReader(opts.StdinData)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("exec start: %w", err)
	}

	// Goroutine: watch context cancellation to escalate from SIGTERM to SIGKILL.
	done := make(chan struct{})
	go func() {
		select {
		case <-timeoutCtx.Done():
			if cmd.Process != nil {
				_ = cmd.Process.Signal(os.Interrupt)
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					_ = cmd.Process.Kill()
				}
			}
		case <-done:
		}
	}()

	err := cmd.Wait()
	close(done)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if timeoutCtx.Err() != nil {
			return nil, NewAbortError("命令超时或被取消")
		} else {
			return nil, fmt.Errorf("exec wait: %w", err)
		}
	}

	return &ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, nil
}

// getShellArgs returns the shell binary and all arguments needed to execute a
// command string. On Windows, PowerShell is preferred over cmd.exe because
// LLMs generate bash-style commands that PowerShell handles better.
func getShellArgs() []string {
	if runtime.GOOS == "windows" {
		// Prefer pwsh (PowerShell 7+) over powershell.exe (5.1).
		if p, err := exec.LookPath("pwsh"); err == nil {
			return []string{p, "-NoProfile", "-NonInteractive", "-Command"}
		}
		if p, err := exec.LookPath("powershell"); err == nil {
			return []string{p, "-NoProfile", "-NonInteractive", "-Command"}
		}
		return []string{"cmd", "/c"}
	}
	// Prefer $SHELL, fall back to /bin/sh.
	if sh := os.Getenv("SHELL"); sh != "" {
		return []string{sh, "-c"}
	}
	return []string{"/bin/sh", "-c"}
}
