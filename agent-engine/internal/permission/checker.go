package permission

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// HookFn is a callback invoked during permission checks to allow hooks to
// modify the permission decision. Returns a PermissionDecision or nil to
// defer to the default logic.
type HookFn func(ctx context.Context, req CheckRequest) (*PermissionDecision, error)

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
	// denialTracking tracks consecutive/total denials for fallback-to-prompting.
	denialTracking DenialTrackingState
	// askFn is called when the user must confirm an operation.
	askFn func(ctx context.Context, tool, desc string) (bool, error)
	// classifier is the auto-mode security classifier (nil = disabled).
	classifier Classifier
	// hookFn is called to let hooks modify permission decisions.
	hookFn HookFn
}

// NewChecker creates a Checker with the given configuration.
func NewChecker(mode Mode, allow, deny []Rule, allowedDirs, deniedCmds []string) *Checker {
	return &Checker{
		mode:           mode,
		allowRules:     allow,
		denyRules:      deny,
		allowedDirs:    allowedDirs,
		deniedCmds:     deniedCmds,
		denialTracking: NewDenialTrackingState(),
	}
}

// SetFailClosed enables fail-closed mode: any tool not explicitly allowed by
// an allow rule is denied in ModeDefault.
func (c *Checker) SetFailClosed(v bool) { c.failClosed = v }

// SetClassifier sets the auto-mode security classifier.
func (c *Checker) SetClassifier(cl Classifier) { c.classifier = cl }

// SetHookFn sets the hook callback for permission checks.
func (c *Checker) SetHookFn(fn HookFn) { c.hookFn = fn }

// DenialTracking returns the current denial tracking state.
func (c *Checker) DenialTracking() DenialTrackingState { return c.denialTracking }

// Denials returns a snapshot of all denial records accumulated so far.
func (c *Checker) Denials() []DenialRecord {
	out := make([]DenialRecord, len(c.denials))
	copy(out, c.denials)
	return out
}

// recordDenial appends a denial record and updates denial tracking.
func (c *Checker) recordDenial(req CheckRequest, reason string) {
	c.denials = append(c.denials, DenialRecord{
		ToolName: req.ToolName,
		Reason:   reason,
		Input:    req.ToolInput,
	})
	c.denialTracking = c.denialTracking.RecordDenial()
}

// recordSuccess resets consecutive denial counter.
func (c *Checker) recordSuccess() {
	c.denialTracking = c.denialTracking.RecordSuccess()
}

// SetAskFunc sets the callback used when user confirmation is required.
func (c *Checker) SetAskFunc(fn func(ctx context.Context, tool, desc string) (bool, error)) {
	c.askFn = fn
}

// Mode returns the current permission mode.
func (c *Checker) Mode() Mode { return c.mode }

// SetMode changes the permission mode.
func (c *Checker) SetMode(m Mode) { c.mode = m }

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

	// 3. Hook integration — hooks may override all other logic.
	if c.hookFn != nil {
		if decision, err := c.hookFn(ctx, req); err != nil {
			return err
		} else if decision != nil {
			switch decision.Type {
			case "allow":
				c.recordSuccess()
				return nil
			case "deny":
				c.recordDenial(req, decision.Message)
				return fmt.Errorf("%s", decision.Message)
				// "ask" falls through to normal flow
			}
		}
	}

	// 4. Hard allow rules short-circuit remaining checks.
	if c.matchesAllowRules(req) {
		c.recordSuccess()
		return nil
	}

	// 5. Bypass mode — allow everything not hard-denied.
	if c.mode == ModeBypassAll {
		c.recordSuccess()
		return nil
	}

	// 6. Plan mode — only read-only tools are allowed.
	if c.mode == ModePlan && !req.IsReadOnly {
		reason := fmt.Sprintf("tool %q is not read-only (plan mode)", req.ToolName)
		c.recordDenial(req, reason)
		return fmt.Errorf("%s", reason)
	}

	// 7. AcceptEdits mode — allow file edit tools without asking.
	if c.mode == ModeAcceptEdits && isFileEditTool(req.ToolName) {
		c.recordSuccess()
		return nil
	}

	// 8. File system safety check: path traversal + allowed-dir constraint.
	if err := c.checkFileSystemSafety(req); err != nil {
		c.recordDenial(req, err.Error())
		return err
	}

	// 9. Shell command denylist.
	if err := c.checkDeniedCommands(req); err != nil {
		c.recordDenial(req, err.Error())
		return err
	}

	// 10. Auto Mode — LLM classifier.
	if c.mode == ModeAutoApprove && c.classifier != nil {
		// Check if denial tracking says we should fall back to prompting.
		if !c.denialTracking.ShouldFallbackToPrompting() {
			result, err := c.classifier.Classify(ctx, ClassifyRequest{
				ToolName:  req.ToolName,
				ToolInput: req.ToolInput,
				Signal:    ctx,
			})
			if err != nil {
				// Classifier error — fail closed.
				c.recordDenial(req, "classifier error: "+err.Error())
				return fmt.Errorf("auto-mode classifier error: %w", err)
			}
			if result.ShouldBlock {
				c.recordDenial(req, result.Reason)
				return fmt.Errorf("auto-mode blocked: %s", result.Reason)
			}
			c.recordSuccess()
			return nil
		}
		// Fallback to prompting when too many denials.
	}

	// 11. DontAsk mode — deny anything not already auto-allowed.
	if c.mode == ModeDontAsk {
		reason := fmt.Sprintf("tool %q denied (dontAsk mode)", req.ToolName)
		c.recordDenial(req, reason)
		return fmt.Errorf("%s", reason)
	}

	// 12. Fail-closed: deny everything not explicitly allowed.
	if c.failClosed && c.mode == ModeDefault {
		reason := fmt.Sprintf("tool %q not explicitly allowed (fail-closed mode)", req.ToolName)
		c.recordDenial(req, reason)
		return fmt.Errorf("%s", reason)
	}

	// 13. Default: ask user (if askFn configured), otherwise allow.
	if c.askFn != nil && c.mode == ModeDefault {
		allowed, err := c.askFn(ctx, req.ToolName, fmt.Sprintf("Allow %s?", req.ToolName))
		if err != nil {
			return err
		}
		if !allowed {
			c.recordDenial(req, "user denied")
			return fmt.Errorf("user denied %q", req.ToolName)
		}
		c.recordSuccess()
	}

	return nil
}

// CheckFull evaluates a permission request and returns a full PermissionDecision.
func (c *Checker) CheckFull(ctx context.Context, req CheckRequest) PermissionDecision {
	err := c.Check(ctx, req)
	if err == nil {
		return PermissionDecision{Type: "allow", UpdatedInput: req.ToolInput}
	}
	return PermissionDecision{Type: "deny", Message: err.Error()}
}

// isFileEditTool reports whether the tool name is a file editing tool.
func isFileEditTool(name string) bool {
	switch name {
	case "Edit", "Write", "MultiEdit", "NotebookEdit",
		"file_edit", "file_write", "multi_edit", "notebook_edit":
		return true
	}
	return false
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
