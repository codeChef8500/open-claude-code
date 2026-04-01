package plugin

import (
	"context"
	"strings"
)

// RegisterBuiltinHooks wires built-in plugin behaviours into a HookEngine.
// Currently ships two read-only review hooks:
//   - code-review:    logs a warning on large file writes
//   - security-review: blocks shell commands containing known dangerous patterns
func RegisterBuiltinHooks(he *HookEngine) {
	he.Register(HookPreToolUse, codeReviewHook)
	he.Register(HookPreToolUse, securityReviewHook)
}

// codeReviewHook emits a warning when a file-write tool call is unusually large.
func codeReviewHook(_ context.Context, p HookPayload) (*HookResult, error) {
	if p.ToolName != "file_write" {
		return nil, nil
	}
	if content, ok := extractStringField(p.ToolInput, "content"); ok {
		if len(content) > 50_000 {
			return &HookResult{
				Block:  false,
				Reason: "code-review: file write is very large (>50k chars) — consider splitting",
			}, nil
		}
	}
	return nil, nil
}

// securityReviewHook blocks bash commands that contain obviously dangerous patterns.
var dangerousPatterns = []string{
	"rm -rf /",
	":(){ :|:& };:",  // fork bomb
	"dd if=/dev/zero of=/dev/",
	"> /dev/sda",
	"mkfs.",
}

func securityReviewHook(_ context.Context, p HookPayload) (*HookResult, error) {
	if p.ToolName != "bash" {
		return nil, nil
	}
	cmd, ok := extractStringField(p.ToolInput, "command")
	if !ok {
		return nil, nil
	}
	lower := strings.ToLower(cmd)
	for _, pat := range dangerousPatterns {
		if strings.Contains(lower, pat) {
			return &HookResult{
				Block:  true,
				Reason: "security-review: command matches dangerous pattern: " + pat,
			}, nil
		}
	}
	return nil, nil
}

// extractStringField attempts to extract a string field from an interface{} that
// may be a map[string]interface{} (as produced by json.Unmarshal).
func extractStringField(v interface{}, field string) (string, bool) {
	if m, ok := v.(map[string]interface{}); ok {
		if val, exists := m[field]; exists {
			if s, ok := val.(string); ok {
				return s, true
			}
		}
	}
	return "", false
}
