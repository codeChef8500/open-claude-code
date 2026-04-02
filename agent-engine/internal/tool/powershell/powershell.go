package powershell

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

const (
	defaultTimeoutMs = 120_000 // 2 minutes
	maxTimeoutMs     = 600_000 // 10 minutes
	maxOutputChars   = 100_000
)

// Input is the JSON input schema for PowerShellTool.
type Input struct {
	Command     string `json:"command"`
	TimeoutMs   int    `json:"timeout,omitempty"`
	Description string `json:"description,omitempty"`
}

// PowerShellTool executes PowerShell commands on Windows.
type PowerShellTool struct {
	tool.BaseTool
	psPath string // cached path to powershell/pwsh
}

// New creates a PowerShellTool. It detects the available PowerShell binary.
func New() *PowerShellTool {
	t := &PowerShellTool{}
	t.psPath = detectPowerShell()
	return t
}

func (t *PowerShellTool) Name() string           { return "PowerShell" }
func (t *PowerShellTool) UserFacingName() string  { return "powershell" }
func (t *PowerShellTool) Description() string {
	return "Execute a PowerShell command and return its output. Available on Windows systems."
}
func (t *PowerShellTool) MaxResultSizeChars() int              { return maxOutputChars }
func (t *PowerShellTool) IsDestructive(_ json.RawMessage) bool { return true }
func (t *PowerShellTool) ShouldDefer() bool                    { return true }
func (t *PowerShellTool) InterruptBehavior() engine.InterruptBehavior {
	return engine.InterruptBehaviorReturn
}

func (t *PowerShellTool) IsEnabled(_ *tool.UseContext) bool {
	return runtime.GOOS == "windows" && t.psPath != ""
}

func (t *PowerShellTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "The PowerShell command to execute."
			},
			"timeout": {
				"type": "integer",
				"description": "Timeout in milliseconds (default 120000, max 600000)."
			},
			"description": {
				"type": "string",
				"description": "Brief description shown to the user."
			}
		},
		"required": ["command"]
	}`)
}

func (t *PowerShellTool) Prompt(_ *tool.UseContext) string {
	return `## PowerShellTool
Execute PowerShell commands on Windows. Commands time out after 2 minutes by default.
- Use PowerShell cmdlets (Get-ChildItem, Select-String, etc.) instead of Unix equivalents.
- Avoid interactive commands that wait for input.
- Use the 'timeout' parameter for long-running operations.
- Prefer non-destructive commands; ask before modifying the system.`
}

func (t *PowerShellTool) IsReadOnly(input json.RawMessage) bool {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return false
	}
	return isReadOnlyPSCommand(in.Command)
}

func (t *PowerShellTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *PowerShellTool) GetActivityDescription(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil || in.Command == "" {
		return "Running PowerShell command"
	}
	cmd := in.Command
	if len(cmd) > 60 {
		cmd = cmd[:60] + "…"
	}
	return "Running PS: " + cmd
}

func (t *PowerShellTool) GetToolUseSummary(input json.RawMessage) string {
	return t.GetActivityDescription(input)
}

func (t *PowerShellTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil // Permission checked externally.
}

func (t *PowerShellTool) Call(ctx context.Context, input json.RawMessage, _ *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	ch := make(chan *engine.ContentBlock, 2)

	go func() {
		defer close(ch)

		var in Input
		if err := json.Unmarshal(input, &in); err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "invalid input: " + err.Error(), IsError: true}
			return
		}
		if in.Command == "" {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "command is required", IsError: true}
			return
		}

		timeoutMs := in.TimeoutMs
		if timeoutMs <= 0 {
			timeoutMs = defaultTimeoutMs
		}
		if timeoutMs > maxTimeoutMs {
			timeoutMs = maxTimeoutMs
		}

		timeout := time.Duration(timeoutMs) * time.Millisecond
		execCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Execute via PowerShell.
		cmd := exec.CommandContext(execCtx, t.psPath, "-NoProfile", "-NonInteractive", "-Command", in.Command)

		output, err := cmd.CombinedOutput()
		text := string(output)

		if err != nil {
			if execCtx.Err() == context.DeadlineExceeded {
				text += fmt.Sprintf("\n\n[Timed out after %dms]", timeoutMs)
			} else if text == "" {
				text = fmt.Sprintf("PowerShell error: %v", err)
			} else {
				text += fmt.Sprintf("\n\n[Exit error: %v]", err)
			}
		}

		// Truncate if needed.
		if len(text) > maxOutputChars {
			text = text[:maxOutputChars] + "\n... [truncated]"
		}

		isErr := err != nil && execCtx.Err() != context.DeadlineExceeded
		ch <- &engine.ContentBlock{
			Type:    engine.ContentTypeText,
			Text:    text,
			IsError: isErr,
		}
	}()

	return ch, nil
}

// detectPowerShell finds the PowerShell binary.
// Prefers pwsh (PowerShell 7+) over powershell.exe (Windows PowerShell 5.1).
func detectPowerShell() string {
	if path, err := exec.LookPath("pwsh"); err == nil {
		return path
	}
	if path, err := exec.LookPath("powershell"); err == nil {
		return path
	}
	if runtime.GOOS == "windows" {
		// Fallback to well-known Windows path.
		return `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
	}
	return ""
}

// readOnlyPSCmdlets are PowerShell cmdlets considered safe/read-only.
var readOnlyPSCmdlets = map[string]bool{
	"get-childitem":  true,
	"get-content":    true,
	"get-item":       true,
	"get-location":   true,
	"get-process":    true,
	"get-service":    true,
	"get-filehash":   true,
	"get-acl":        true,
	"test-path":      true,
	"resolve-path":   true,
	"select-string":  true,
	"format-hex":     true,
	"measure-object": true,
	"write-output":   true,
	"write-host":     true,
	"get-date":       true,
	"get-help":       true,
	"get-command":    true,
	"get-module":     true,
	"get-variable":   true,
	"get-alias":      true,
	"where-object":   true,
	"select-object":  true,
	"sort-object":    true,
	"group-object":   true,
	"format-list":    true,
	"format-table":   true,
}

// isReadOnlyPSCommand checks if a PowerShell command is a read-only cmdlet.
func isReadOnlyPSCommand(command string) bool {
	// Simple heuristic: check if the first token is a known read-only cmdlet.
	parts := strings.Fields(strings.TrimSpace(command))
	if len(parts) == 0 {
		return false
	}
	first := strings.ToLower(parts[0])
	return readOnlyPSCmdlets[first]
}
