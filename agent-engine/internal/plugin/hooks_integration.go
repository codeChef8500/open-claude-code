package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PluginHookMatcher pairs matcher criteria with hook handlers from a plugin.
type PluginHookMatcher struct {
	// PluginName is the source plugin.
	PluginName string
	// PluginRoot is the absolute path to the plugin directory.
	PluginRoot string
	// ToolNamePattern is an optional tool name glob for pre/post_tool_use hooks.
	ToolNamePattern string
	// Command is the shell command to execute for this hook.
	Command string
	// Timeout for hook command execution.
	Timeout time.Duration
}

// PluginHookRegistry manages hooks loaded from manifest-based plugins.
type PluginHookRegistry struct {
	mu       sync.RWMutex
	matchers map[HookType][]PluginHookMatcher
}

// NewPluginHookRegistry creates an empty plugin hook registry.
func NewPluginHookRegistry() *PluginHookRegistry {
	return &PluginHookRegistry{
		matchers: make(map[HookType][]PluginHookMatcher),
	}
}

// RegisterPluginHooks loads hooks from a loaded plugin's hooks config and
// registers them into both the PluginHookRegistry and a HookEngine.
func (r *PluginHookRegistry) RegisterPluginHooks(plugin *LoadedPlugin, he *HookEngine) error {
	if plugin.HooksConfig == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	hookTypes := []HookType{
		HookPreToolUse, HookPostToolUse, HookStop,
		HookUserPromptSubmit, HookSessionStart, HookNotification,
	}

	for _, ht := range hookTypes {
		entries := extractHookEntries(plugin.HooksConfig, string(ht))
		for _, entry := range entries {
			matcher := PluginHookMatcher{
				PluginName:      plugin.Name,
				PluginRoot:      plugin.Path,
				ToolNamePattern: entry.matcher,
				Command:         entry.command,
				Timeout:         entry.timeout,
			}

			r.matchers[ht] = append(r.matchers[ht], matcher)

			// Also register as a HookHandler in the engine.
			handler := buildPluginHookHandler(matcher, plugin)
			he.Register(ht, handler)
		}
	}

	return nil
}

// ClearPluginHooks removes all hooks from a specific plugin.
func (r *PluginHookRegistry) ClearPluginHooks(pluginName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for ht, matchers := range r.matchers {
		filtered := make([]PluginHookMatcher, 0, len(matchers))
		for _, m := range matchers {
			if m.PluginName != pluginName {
				filtered = append(filtered, m)
			}
		}
		r.matchers[ht] = filtered
	}
}

// PruneRemovedPluginHooks removes hooks for plugins no longer in the enabled set.
func (r *PluginHookRegistry) PruneRemovedPluginHooks(enabledPlugins []*LoadedPlugin) {
	enabled := make(map[string]struct{})
	for _, p := range enabledPlugins {
		enabled[p.Name] = struct{}{}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for ht, matchers := range r.matchers {
		filtered := make([]PluginHookMatcher, 0, len(matchers))
		for _, m := range matchers {
			if _, ok := enabled[m.PluginName]; ok {
				filtered = append(filtered, m)
			}
		}
		r.matchers[ht] = filtered
	}
}

// GetMatchers returns all registered matchers for a hook type.
func (r *PluginHookRegistry) GetMatchers(ht HookType) []PluginHookMatcher {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]PluginHookMatcher{}, r.matchers[ht]...)
}

// LoadPluginHooks reads hook configs from all enabled plugins and returns
// a map of HookType → matchers.
func LoadPluginHooks(plugins []*LoadedPlugin) map[HookType][]PluginHookMatcher {
	result := make(map[HookType][]PluginHookMatcher)

	for _, plugin := range plugins {
		if plugin.HooksConfig == nil {
			continue
		}

		hookTypes := []HookType{
			HookPreToolUse, HookPostToolUse, HookStop,
			HookUserPromptSubmit, HookSessionStart, HookNotification,
		}

		for _, ht := range hookTypes {
			entries := extractHookEntries(plugin.HooksConfig, string(ht))
			for _, entry := range entries {
				result[ht] = append(result[ht], PluginHookMatcher{
					PluginName:      plugin.Name,
					PluginRoot:      plugin.Path,
					ToolNamePattern: entry.matcher,
					Command:         entry.command,
					Timeout:         entry.timeout,
				})
			}
		}
	}

	return result
}

// ── Internal helpers ─────────────────────────────────────────────────────────

type hookEntry struct {
	matcher string
	command string
	timeout time.Duration
}

// extractHookEntries parses hook entries from the hooks config for a given type.
// The format follows claude-code-main's hooks settings:
//
//	{ "pre_tool_use": [{ "matcher": "Bash", "hooks": [{ "type": "command", "command": "..." }] }] }
func extractHookEntries(config map[string]interface{}, hookType string) []hookEntry {
	raw, ok := config[hookType]
	if !ok {
		return nil
	}

	// Marshal/unmarshal for flexible parsing.
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}

	var entries []hookEntry

	// Try array of hook groups.
	var groups []struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Type    string `json:"type"`
			Command string `json:"command"`
			Timeout int    `json:"timeout"`
		} `json:"hooks"`
	}
	if json.Unmarshal(data, &groups) == nil {
		for _, g := range groups {
			for _, h := range g.Hooks {
				if h.Type != "command" || h.Command == "" {
					continue
				}
				timeout := 10 * time.Second
				if h.Timeout > 0 {
					timeout = time.Duration(h.Timeout) * time.Second
				}
				entries = append(entries, hookEntry{
					matcher: g.Matcher,
					command: h.Command,
					timeout: timeout,
				})
			}
		}
		return entries
	}

	return entries
}

// buildPluginHookHandler creates a HookHandler from a PluginHookMatcher.
func buildPluginHookHandler(matcher PluginHookMatcher, plugin *LoadedPlugin) HookHandler {
	return func(ctx context.Context, payload HookPayload) (*HookResult, error) {
		// Check tool name matcher for pre/post_tool_use hooks.
		if matcher.ToolNamePattern != "" && payload.ToolName != "" {
			if !matchToolName(payload.ToolName, matcher.ToolNamePattern) {
				return nil, nil
			}
		}

		// Build environment variables.
		env := buildHookEnv(payload, plugin)

		// Prepare hook input as JSON on stdin.
		input, _ := json.Marshal(map[string]interface{}{
			"hook_type":  string(payload.Type),
			"tool_name":  payload.ToolName,
			"tool_input": payload.ToolInput,
			"result":     payload.Result,
			"session_id": payload.SessionID,
			"message":    payload.Message,
		})

		// Execute the command.
		timeout := matcher.Timeout
		if timeout == 0 {
			timeout = 10 * time.Second
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, resolveHookShell())
		cmd.Args = append(cmd.Args, hookShellFlag(), matcher.Command)
		cmd.Dir = matcher.PluginRoot
		cmd.Env = append(os.Environ(), env...)
		cmd.Stdin = strings.NewReader(string(input))

		output, err := cmd.CombinedOutput()
		if err != nil {
			slog.Warn("plugin hook command failed",
				slog.String("plugin", matcher.PluginName),
				slog.String("command", matcher.Command),
				slog.Any("err", err))
			return nil, err
		}

		// Parse output as HookResult.
		return parsePluginHookOutput(output)
	}
}

// parsePluginHookOutput parses the JSON output from a plugin hook command.
func parsePluginHookOutput(output []byte) (*HookResult, error) {
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return nil, nil
	}

	var result struct {
		Block    bool        `json:"block"`
		Reason   string      `json:"reason"`
		Modified interface{} `json:"modified"`
	}
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		// Non-JSON output is not an error — the hook just had no structured response.
		return nil, nil
	}

	return &HookResult{
		Block:    result.Block,
		Reason:   result.Reason,
		Modified: result.Modified,
	}, nil
}

// matchToolName does simple glob matching for tool name patterns.
func matchToolName(toolName, pattern string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	// Exact match.
	if strings.EqualFold(toolName, pattern) {
		return true
	}
	// Simple prefix wildcard: "Bash*" matches "BashTool".
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(strings.ToLower(toolName), strings.ToLower(prefix))
	}
	return false
}

// buildHookEnv builds environment variables for a plugin hook command.
func buildHookEnv(payload HookPayload, plugin *LoadedPlugin) []string {
	env := []string{
		fmt.Sprintf("CLAUDE_HOOK_TYPE=%s", payload.Type),
		fmt.Sprintf("CLAUDE_PLUGIN_NAME=%s", plugin.Name),
		fmt.Sprintf("CLAUDE_PLUGIN_ROOT=%s", plugin.Path),
	}
	if payload.ToolName != "" {
		env = append(env, fmt.Sprintf("CLAUDE_TOOL_NAME=%s", payload.ToolName))
	}
	if payload.SessionID != "" {
		env = append(env, fmt.Sprintf("CLAUDE_SESSION_ID=%s", payload.SessionID))
	}

	// Add user config values as CLAUDE_PLUGIN_OPTION_KEY env vars.
	if plugin.UserConfigValues != nil {
		for k, v := range plugin.UserConfigValues {
			key := strings.ToUpper(k)
			env = append(env, fmt.Sprintf("CLAUDE_PLUGIN_OPTION_%s=%v", key, v))
		}
	}

	// Plugin data directory.
	home, _ := os.UserHomeDir()
	if home != "" {
		dataDir := filepath.Join(home, ".claude", "plugin-data", plugin.Name)
		env = append(env, fmt.Sprintf("CLAUDE_PLUGIN_DATA=%s", dataDir))
	}

	return env
}

// resolveHookShell returns the shell binary for hook execution.
func resolveHookShell() string {
	if os.Getenv("COMSPEC") != "" {
		return "powershell"
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "bash"
}

// hookShellFlag returns the command flag for the shell.
func hookShellFlag() string {
	shell := resolveHookShell()
	if strings.Contains(shell, "powershell") || strings.Contains(shell, "pwsh") {
		return "-Command"
	}
	return "-c"
}
