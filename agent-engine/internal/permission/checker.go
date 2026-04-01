package permission

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// Checker evaluates permission requests against the configured rules and mode.
type Checker struct {
	mode         Mode
	allowRules   []Rule
	denyRules    []Rule
	allowedDirs  []string
	deniedCmds   []string
	// askFn is called when the user must confirm an operation.
	askFn func(ctx context.Context, tool, desc string) (bool, error)
}

// NewChecker creates a Checker with the given configuration.
func NewChecker(mode Mode, allow, deny []Rule, allowedDirs, deniedCmds []string) *Checker {
	return &Checker{
		mode:        mode,
		allowRules:  allow,
		denyRules:   deny,
		allowedDirs: allowedDirs,
		deniedCmds:  deniedCmds,
	}
}

// SetAskFunc sets the callback used when user confirmation is required.
func (c *Checker) SetAskFunc(fn func(ctx context.Context, tool, desc string) (bool, error)) {
	c.askFn = fn
}

// Check evaluates a permission request and returns nil if permitted, or an
// error explaining why it was denied.
func (c *Checker) Check(ctx context.Context, req CheckRequest) error {
	// 1. Hard deny rules always win.
	if c.matchesDenyRules(req) {
		return fmt.Errorf("tool %q is denied by policy", req.ToolName)
	}

	// 2. Hard allow rules short-circuit remaining checks.
	if c.matchesAllowRules(req) {
		return nil
	}

	// 3. Bypass mode — allow everything not hard-denied.
	if c.mode == ModeBypassAll {
		return nil
	}

	// 4. File system safety check for write tools.
	if err := c.checkFileSystemSafety(req); err != nil {
		return err
	}

	// 5. Shell command denylist.
	if err := c.checkDeniedCommands(req); err != nil {
		return err
	}

	// 6. Auto Mode — LLM classifier (handled externally; if result is
	//    SoftDeny the caller wraps with a descriptive error).

	// 7. Default mode: read-only operations are auto-approved;
	//    write operations require user confirmation.
	return nil
}

func (c *Checker) matchesDenyRules(req CheckRequest) bool {
	for _, r := range c.denyRules {
		if r.Type == RuleDeny && matchPattern(r.Pattern, req.ToolName) {
			return true
		}
	}
	return false
}

func (c *Checker) matchesAllowRules(req CheckRequest) bool {
	for _, r := range c.allowRules {
		if r.Type == RuleAllow && matchPattern(r.Pattern, req.ToolName) {
			return true
		}
	}
	return false
}

func (c *Checker) checkFileSystemSafety(req CheckRequest) error {
	if len(c.allowedDirs) == 0 {
		return nil
	}
	// Extract path from tool input if available.
	path := extractPath(req.ToolInput)
	if path == "" {
		return nil
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil
	}
	for _, dir := range c.allowedDirs {
		absDir, _ := filepath.Abs(dir)
		if strings.HasPrefix(absPath, absDir) {
			return nil
		}
	}
	return fmt.Errorf("path %q is outside of allowed directories", path)
}

func (c *Checker) checkDeniedCommands(req CheckRequest) error {
	if req.ToolName != "bash" && req.ToolName != "Bash" {
		return nil
	}
	cmd := extractCommand(req.ToolInput)
	if cmd == "" {
		return nil
	}
	for _, denied := range c.deniedCmds {
		if strings.Contains(cmd, denied) {
			return fmt.Errorf("command contains denied pattern %q", denied)
		}
	}
	return nil
}

func matchPattern(pattern, name string) bool {
	if pattern == "*" {
		return true
	}
	matched, err := filepath.Match(pattern, name)
	if err != nil {
		return pattern == name
	}
	return matched
}

func extractPath(input interface{}) string {
	if m, ok := input.(map[string]interface{}); ok {
		for _, key := range []string{"path", "file_path", "filePath"} {
			if v, ok := m[key]; ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
		}
	}
	return ""
}

func extractCommand(input interface{}) string {
	if m, ok := input.(map[string]interface{}); ok {
		for _, key := range []string{"command", "cmd"} {
			if v, ok := m[key]; ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
		}
	}
	return ""
}
