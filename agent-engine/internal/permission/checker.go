package permission

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// Checker evaluates permission requests against the configured rules and mode.
type Checker struct {
	mode        Mode
	allowRules  []Rule
	denyRules   []Rule
	allowedDirs []string
	deniedCmds  []string
	// failClosed causes the checker to deny any operation not explicitly allowed
	// when mode == ModeDefault (instead of the previous open-by-default behavior).
	failClosed bool
	// denials accumulates denial records for audit and diagnostics.
	denials []DenialRecord
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

// SetFailClosed enables fail-closed mode: any tool not explicitly allowed by
// an allow rule is denied in ModeDefault.
func (c *Checker) SetFailClosed(v bool) { c.failClosed = v }

// Denials returns a snapshot of all denial records accumulated so far.
func (c *Checker) Denials() []DenialRecord {
	out := make([]DenialRecord, len(c.denials))
	copy(out, c.denials)
	return out
}

// recordDenial appends a denial record.
func (c *Checker) recordDenial(req CheckRequest, reason string) {
	c.denials = append(c.denials, DenialRecord{
		ToolName: req.ToolName,
		Reason:   reason,
		Input:    req.ToolInput,
	})
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
		reason := fmt.Sprintf("tool %q is denied by policy", req.ToolName)
		c.recordDenial(req, reason)
		return fmt.Errorf("%s", reason)
	}

	// 2. Dangerous shell pattern check (fail-hard regardless of rules).
	if err := c.checkDangerousPatterns(req); err != nil {
		c.recordDenial(req, err.Error())
		return err
	}

	// 3. Hard allow rules short-circuit remaining checks.
	if c.matchesAllowRules(req) {
		return nil
	}

	// 4. Bypass mode — allow everything not hard-denied.
	if c.mode == ModeBypassAll {
		return nil
	}

	// 5. File system safety check: path traversal + allowed-dir constraint.
	if err := c.checkFileSystemSafety(req); err != nil {
		c.recordDenial(req, err.Error())
		return err
	}

	// 6. Shell command denylist.
	if err := c.checkDeniedCommands(req); err != nil {
		c.recordDenial(req, err.Error())
		return err
	}

	// 7. Auto Mode — LLM classifier handled externally.

	// 8. Fail-closed: deny everything not explicitly allowed.
	if c.failClosed && c.mode == ModeDefault {
		reason := fmt.Sprintf("tool %q not explicitly allowed (fail-closed mode)", req.ToolName)
		c.recordDenial(req, reason)
		return fmt.Errorf("%s", reason)
	}

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
	// Extract path from tool input if available.
	path := extractPath(req.ToolInput)
	if path == "" {
		return nil
	}

	// Block path traversal sequences before resolving.
	if strings.Contains(path, "..") {
		return fmt.Errorf("path %q contains traversal sequence '..'", path)
	}

	if len(c.allowedDirs) == 0 {
		return nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil
	}
	// Ensure the resolved path is within an allowed directory.
	for _, dir := range c.allowedDirs {
		absDir, _ := filepath.Abs(dir)
		// Add separator to prevent prefix-matching a sibling dir.
		if absPath == absDir || strings.HasPrefix(absPath, absDir+string(filepath.Separator)) {
			return nil
		}
	}
	return fmt.Errorf("path %q is outside of allowed directories", path)
}

// checkDangerousPatterns rejects shell commands that contain well-known
// destructive patterns, regardless of other allow rules.
func (c *Checker) checkDangerousPatterns(req CheckRequest) error {
	if req.ToolName != "bash" && req.ToolName != "Bash" {
		return nil
	}
	cmd := extractCommand(req.ToolInput)
	if cmd == "" {
		return nil
	}
	lower := strings.ToLower(cmd)
	for _, pat := range DangerousShellPatterns {
		if strings.Contains(lower, strings.ToLower(pat)) {
			return fmt.Errorf("command contains dangerous pattern %q", pat)
		}
	}
	return nil
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
