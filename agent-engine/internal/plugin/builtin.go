package plugin

import (
	"context"
	"strings"
	"sync"

	"github.com/wall-ai/agent-engine/internal/skill"
)

// ── Builtin Plugin Registry ─────────────────────────────────────────────────

var (
	builtinDefsMu sync.RWMutex
	builtinDefs   []BuiltinPluginDefinition
)

// RegisterBuiltinPlugin adds a builtin plugin definition.
func RegisterBuiltinPlugin(def BuiltinPluginDefinition) {
	builtinDefsMu.Lock()
	defer builtinDefsMu.Unlock()
	builtinDefs = append(builtinDefs, def)
}

// GetBuiltinPlugins returns enabled and disabled builtin plugins based on
// the user's enabled map. If enabledMap is nil, defaults are used.
func GetBuiltinPlugins(enabledMap map[string]bool) (enabled, disabled []BuiltinPluginDefinition) {
	builtinDefsMu.RLock()
	defer builtinDefsMu.RUnlock()

	for _, def := range builtinDefs {
		if def.IsAvailable != nil && !def.IsAvailable() {
			continue
		}

		isEnabled := def.DefaultEnabled
		if enabledMap != nil {
			if v, ok := enabledMap[def.Name]; ok {
				isEnabled = v
			}
		}

		if isEnabled {
			enabled = append(enabled, def)
		} else {
			disabled = append(disabled, def)
		}
	}
	return
}

// GetBuiltinPluginSkills converts skills from enabled builtin plugins to Skill objects.
func GetBuiltinPluginSkills(enabledMap map[string]bool) []*skill.Skill {
	enabled, _ := GetBuiltinPlugins(enabledMap)
	var skills []*skill.Skill
	for _, def := range enabled {
		for _, bs := range def.Skills {
			userInvocable := true
			s := &skill.Skill{
				Meta: skill.SkillMeta{
					Name:          def.Name + ":" + bs.Name,
					Description:   bs.Description,
					WhenToUse:     bs.WhenToUse,
					AllowedTools:  bs.AllowedTools,
					UserInvocable: &userInvocable,
					Source:        "builtin",
					LoadedFrom:    "builtin:" + def.Name,
				},
				Prompt: bs.Prompt,
				RawMD:  bs.Prompt,
			}
			skills = append(skills, s)
		}
	}
	return skills
}

// GetBuiltinPluginHooks returns merged hooks from all enabled builtin plugins.
func GetBuiltinPluginHooks(enabledMap map[string]bool) map[HookType][]HookHandler {
	enabled, _ := GetBuiltinPlugins(enabledMap)
	result := make(map[HookType][]HookHandler)
	for _, def := range enabled {
		for ht, handlers := range def.Hooks {
			result[ht] = append(result[ht], handlers...)
		}
	}
	return result
}

// RegisterBuiltinHooks wires all enabled builtin plugin hooks into a HookEngine.
// Also registers the legacy code-review and security-review hooks.
func RegisterBuiltinHooks(he *HookEngine) {
	// Legacy hooks (always active).
	he.Register(HookPreToolUse, codeReviewHook)
	he.Register(HookPreToolUse, securityReviewHook)

	// Hooks from enabled builtin plugins.
	hooks := GetBuiltinPluginHooks(nil)
	for ht, handlers := range hooks {
		for _, h := range handlers {
			he.Register(ht, h)
		}
	}
}

// InitBuiltinPlugins registers the default set of builtin plugins.
// Call once at startup.
func InitBuiltinPlugins() {
	RegisterBuiltinPlugin(safetyReviewPlugin)
}

// ResetBuiltinPlugins clears all registered builtin plugins. For testing.
func ResetBuiltinPlugins() {
	builtinDefsMu.Lock()
	defer builtinDefsMu.Unlock()
	builtinDefs = nil
}

// ── Builtin Plugin Definitions ──────────────────────────────────────────────

var safetyReviewPlugin = BuiltinPluginDefinition{
	Name:           "safety-review",
	Description:    "Built-in code review and security review hooks",
	Version:        "1.0.0",
	DefaultEnabled: true,
	Hooks: map[HookType][]HookHandler{
		HookPreToolUse: {codeReviewHook, securityReviewHook},
	},
}

// ── Legacy hook implementations ─────────────────────────────────────────────

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
	":(){ :|:& };:", // fork bomb
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
