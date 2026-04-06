package util

import (
	"os"
	"strings"
	"sync"
)

// ────────────────────────────────────────────────────────────────────────────
// Feature Flags — runtime feature toggles for gradual rollout and experimentation.
// Aligned with claude-code-main's feature flag patterns.
// ────────────────────────────────────────────────────────────────────────────

// FeatureFlag identifies a toggleable feature.
type FeatureFlag string

const (
	// FlagExtendedThinking enables extended thinking mode.
	FlagExtendedThinking FeatureFlag = "extended_thinking"
	// FlagAutoCompact enables auto-compaction.
	FlagAutoCompact FeatureFlag = "auto_compact"
	// FlagMicroCompact enables micro-compaction pass.
	FlagMicroCompact FeatureFlag = "micro_compact"
	// FlagPromptCache enables prompt caching.
	FlagPromptCache FeatureFlag = "prompt_cache"
	// FlagParallelTools enables parallel tool execution.
	FlagParallelTools FeatureFlag = "parallel_tools"
	// FlagStreamingToolExec enables streaming tool execution.
	FlagStreamingToolExec FeatureFlag = "streaming_tool_exec"
	// FlagToolResultBudget enables tool result disk persistence.
	FlagToolResultBudget FeatureFlag = "tool_result_budget"
	// FlagReactiveCompact enables reactive compaction on PTL errors.
	FlagReactiveCompact FeatureFlag = "reactive_compact"
	// FlagYoloClassifier enables the rule-based auto-mode classifier.
	FlagYoloClassifier FeatureFlag = "yolo_classifier"
	// FlagSubagents enables sub-agent spawning.
	FlagSubagents FeatureFlag = "subagents"
	// FlagTeamMailbox enables teammate mailbox messaging.
	FlagTeamMailbox FeatureFlag = "team_mailbox"
	// FlagHooks enables hook execution.
	FlagHooks FeatureFlag = "hooks"
	// FlagStructuredOutput enables structured output enforcement hooks.
	FlagStructuredOutput FeatureFlag = "structured_output"
	// FlagAuditLog enables permission audit logging.
	FlagAuditLog FeatureFlag = "audit_log"
	// FlagSessionMemory enables session memory extraction.
	FlagSessionMemory FeatureFlag = "session_memory"
	// FlagMCP enables MCP server integration.
	FlagMCP FeatureFlag = "mcp"
	// FlagSkillDiscovery enables dynamic skill discovery.
	FlagSkillDiscovery FeatureFlag = "skill_discovery"
	// FlagMultiLineInput enables multi-line input in TUI.
	FlagMultiLineInput FeatureFlag = "multi_line_input"

	// ── Command-gating flags (aligned with claude-code-main feature()) ──

	// FlagProactive enables the /proactive command.
	FlagProactive FeatureFlag = "proactive"
	// FlagKairos enables Kairos-related commands (/assistant, /proactive, /brief).
	FlagKairos FeatureFlag = "kairos"
	// FlagKairosBrief enables the /brief command.
	FlagKairosBrief FeatureFlag = "kairos_brief"
	// FlagBridgeMode enables the /bridge command.
	FlagBridgeMode FeatureFlag = "bridge_mode"
	// FlagVoiceMode enables the /voice command.
	FlagVoiceMode FeatureFlag = "voice_mode"
	// FlagHistorySnip enables the /force-snip command.
	FlagHistorySnip FeatureFlag = "history_snip"
	// FlagWorkflowScripts enables the /workflows command.
	FlagWorkflowScripts FeatureFlag = "workflow_scripts"
	// FlagKairosGithubWebhooks enables the /subscribe-pr command.
	FlagKairosGithubWebhooks FeatureFlag = "kairos_github_webhooks"
	// FlagUltraplan enables the /ultraplan command.
	FlagUltraplan FeatureFlag = "ultraplan"
	// FlagTorch enables the /torch command.
	FlagTorch FeatureFlag = "torch"
	// FlagUdsInbox enables the /peers command.
	FlagUdsInbox FeatureFlag = "uds_inbox"
	// FlagForkSubagent enables the /fork command.
	FlagForkSubagent FeatureFlag = "fork_subagent"
	// FlagBuddy enables the /buddy command.
	FlagBuddy FeatureFlag = "buddy"
	// FlagDaemon enables daemon-mode features.
	FlagDaemon FeatureFlag = "daemon"
	// FlagCcrRemoteSetup enables the /remote-setup (web) command.
	FlagCcrRemoteSetup FeatureFlag = "ccr_remote_setup"
	// FlagMcpSkills enables MCP skill commands.
	FlagMcpSkills FeatureFlag = "mcp_skills"
	// FlagExperimentalSkillSearch enables experimental skill search indexing.
	FlagExperimentalSkillSearch FeatureFlag = "experimental_skill_search"
	// FlagNewInit enables the new multi-phase /init prompt.
	FlagNewInit FeatureFlag = "new_init"
	// FlagCommitAttribution enables commit attribution texts.
	FlagCommitAttribution FeatureFlag = "commit_attribution"
	// FlagPromptCacheBreakDetection enables prompt cache break detection.
	FlagPromptCacheBreakDetection FeatureFlag = "prompt_cache_break_detection"
	// FlagCoordinatorMode enables coordinator mode for multi-agent.
	FlagCoordinatorMode FeatureFlag = "coordinator_mode"
)

// AllFeatureFlags lists all known feature flags.
var AllFeatureFlags = []FeatureFlag{
	FlagExtendedThinking, FlagAutoCompact, FlagMicroCompact,
	FlagPromptCache, FlagParallelTools, FlagStreamingToolExec,
	FlagToolResultBudget, FlagReactiveCompact, FlagYoloClassifier,
	FlagSubagents, FlagTeamMailbox, FlagHooks, FlagStructuredOutput,
	FlagAuditLog, FlagSessionMemory, FlagMCP, FlagSkillDiscovery,
	FlagMultiLineInput,
	// Command-gating flags
	FlagProactive, FlagKairos, FlagKairosBrief, FlagBridgeMode,
	FlagVoiceMode, FlagHistorySnip, FlagWorkflowScripts,
	FlagKairosGithubWebhooks, FlagUltraplan, FlagTorch, FlagUdsInbox,
	FlagForkSubagent, FlagBuddy, FlagDaemon, FlagCcrRemoteSetup,
	FlagMcpSkills, FlagExperimentalSkillSearch, FlagNewInit,
	FlagCommitAttribution, FlagPromptCacheBreakDetection, FlagCoordinatorMode,
}

// defaultEnabledFlags are flags that are on by default.
var defaultEnabledFlags = map[FeatureFlag]bool{
	FlagAutoCompact:     true,
	FlagMicroCompact:    true,
	FlagPromptCache:     true,
	FlagParallelTools:   true,
	FlagReactiveCompact: true,
	FlagYoloClassifier:  true,
	FlagHooks:           true,
	FlagAuditLog:        true,
	FlagSessionMemory:   true,
	FlagMCP:             true,
}

// FeatureFlagStore manages feature flag state.
type FeatureFlagStore struct {
	mu    sync.RWMutex
	flags map[FeatureFlag]bool
}

// NewFeatureFlagStore creates a store with default flag values.
func NewFeatureFlagStore() *FeatureFlagStore {
	store := &FeatureFlagStore{
		flags: make(map[FeatureFlag]bool, len(AllFeatureFlags)),
	}
	// Load defaults.
	for _, f := range AllFeatureFlags {
		store.flags[f] = defaultEnabledFlags[f]
	}
	// Apply environment overrides.
	store.loadFromEnv()
	return store
}

// IsEnabled returns whether a flag is currently enabled.
func (s *FeatureFlagStore) IsEnabled(flag FeatureFlag) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.flags[flag]
}

// Enable turns on a feature flag.
func (s *FeatureFlagStore) Enable(flag FeatureFlag) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flags[flag] = true
}

// Disable turns off a feature flag.
func (s *FeatureFlagStore) Disable(flag FeatureFlag) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flags[flag] = false
}

// Set sets a flag to a specific value.
func (s *FeatureFlagStore) Set(flag FeatureFlag, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flags[flag] = enabled
}

// Snapshot returns a copy of all flag states.
func (s *FeatureFlagStore) Snapshot() map[FeatureFlag]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[FeatureFlag]bool, len(s.flags))
	for k, v := range s.flags {
		out[k] = v
	}
	return out
}

// LoadFromConfig applies feature flags from the config map.
// Keys should be feature flag names, values should be booleans.
func (s *FeatureFlagStore) LoadFromConfig(flags map[string]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range flags {
		s.flags[FeatureFlag(k)] = v
	}
}

// loadFromEnv reads AGENT_ENGINE_FF_* environment variables.
// AGENT_ENGINE_FF_EXTENDED_THINKING=1 enables FlagExtendedThinking, etc.
func (s *FeatureFlagStore) loadFromEnv() {
	for _, flag := range AllFeatureFlags {
		envKey := "AGENT_ENGINE_FF_" + strings.ToUpper(string(flag))
		if v := os.Getenv(envKey); v != "" {
			switch strings.ToLower(v) {
			case "1", "true", "yes", "on":
				s.flags[flag] = true
			case "0", "false", "no", "off":
				s.flags[flag] = false
			}
		}
	}
}

// Summary returns a human-readable feature flags summary.
func (s *FeatureFlagStore) Summary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var sb strings.Builder
	sb.WriteString("Feature Flags:\n")
	for _, f := range AllFeatureFlags {
		status := "off"
		if s.flags[f] {
			status = "on"
		}
		sb.WriteString("  ")
		sb.WriteString(string(f))
		sb.WriteString(": ")
		sb.WriteString(status)
		sb.WriteString("\n")
	}
	return sb.String()
}
