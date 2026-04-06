package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/util"
)

// ──────────────────────────────────────────────────────────────────────────────
// Feature-gated commands.
// These commands are only enabled when their corresponding feature flags are on.
// Aligned with claude-code-main commands.ts conditional requires.
// ──────────────────────────────────────────────────────────────────────────────

// ─── /proactive ─────────────────────────────────────────────────────────────
// Gated: feature('PROACTIVE') || feature('KAIROS')

type ProactiveCommand struct{ BasePromptCommand }

func (c *ProactiveCommand) Name() string { return "proactive" }
func (c *ProactiveCommand) Description() string {
	return "Enable proactive suggestions from the assistant"
}
func (c *ProactiveCommand) Type() CommandType { return CommandTypePrompt }
func (c *ProactiveCommand) IsEnabled(_ *ExecContext) bool {
	flags := util.NewFeatureFlagStore()
	return flags.IsEnabled(util.FlagProactive) || flags.IsEnabled(util.FlagKairos)
}
func (c *ProactiveCommand) PromptContent(args []string, _ *ExecContext) (string, error) {
	return `## Proactive Mode

You are now in proactive mode. Instead of waiting for explicit instructions, you should:

1. Analyze the current codebase state and identify potential improvements
2. Look for bugs, code smells, and optimization opportunities
3. Suggest refactoring where it would significantly improve code quality
4. Identify missing tests or documentation
5. Look for security vulnerabilities

Be proactive but judicious — focus on high-impact suggestions. Present your findings
organized by severity (critical → improvement → suggestion).`, nil
}

// ─── /brief ─────────────────────────────────────────────────────────────────
// Gated: feature('KAIROS') || feature('KAIROS_BRIEF')

type BriefCommand struct{ BasePromptCommand }

func (c *BriefCommand) Name() string        { return "brief" }
func (c *BriefCommand) Description() string { return "Get a brief project status update" }
func (c *BriefCommand) Type() CommandType   { return CommandTypePrompt }
func (c *BriefCommand) IsEnabled(_ *ExecContext) bool {
	flags := util.NewFeatureFlagStore()
	return flags.IsEnabled(util.FlagKairos) || flags.IsEnabled(util.FlagKairosBrief)
}
func (c *BriefCommand) PromptContent(_ []string, ectx *ExecContext) (string, error) {
	prompt := `## Project Brief

Provide a concise status briefing for the current project:

- Git status:
` + "```\n!`git status --short`\n```" + `

- Recent commits:
` + "```\n!`git log --oneline -5`\n```" + `

Summarize:
1. What has changed recently
2. Current branch status
3. Any pending work or uncommitted changes
4. Potential issues or blockers`

	if ectx != nil && ectx.Services != nil && ectx.Services.Shell != nil {
		prompt, _ = ExecuteShellCommandsInPrompt(
			context.Background(), prompt, ectx.WorkDir, ectx.Services.Shell, 15,
		)
	}
	return prompt, nil
}

// ─── /assistant ─────────────────────────────────────────────────────────────
// Gated: feature('KAIROS')

type AssistantCommand struct{ BaseCommand }

func (c *AssistantCommand) Name() string        { return "assistant" }
func (c *AssistantCommand) Description() string { return "Configure the assistant agent" }
func (c *AssistantCommand) Type() CommandType   { return CommandTypeInteractive }
func (c *AssistantCommand) IsEnabled(_ *ExecContext) bool {
	return util.NewFeatureFlagStore().IsEnabled(util.FlagKairos)
}
func (c *AssistantCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{
		Component: "assistant",
		Data:      map[string]interface{}{"args": args},
	}, nil
}

// ─── /bridge ────────────────────────────────────────────────────────────────
// Gated: feature('BRIDGE_MODE')

type BridgeCommand struct{ BaseCommand }

func (c *BridgeCommand) Name() string        { return "bridge" }
func (c *BridgeCommand) Description() string { return "Configure bridge mode for IDE integration" }
func (c *BridgeCommand) Type() CommandType   { return CommandTypeInteractive }
func (c *BridgeCommand) IsEnabled(_ *ExecContext) bool {
	return util.NewFeatureFlagStore().IsEnabled(util.FlagBridgeMode)
}
func (c *BridgeCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{
		Component: "bridge",
		Data:      map[string]interface{}{"args": args},
	}, nil
}

// ─── /remote-control-server ─────────────────────────────────────────────────
// Gated: feature('DAEMON') && feature('BRIDGE_MODE')

type RemoteControlServerCommand struct{ BaseCommand }

func (c *RemoteControlServerCommand) Name() string        { return "remote-control-server" }
func (c *RemoteControlServerCommand) Description() string { return "Start the remote control server" }
func (c *RemoteControlServerCommand) Type() CommandType   { return CommandTypeLocal }
func (c *RemoteControlServerCommand) IsHidden() bool      { return true }
func (c *RemoteControlServerCommand) IsEnabled(_ *ExecContext) bool {
	flags := util.NewFeatureFlagStore()
	return flags.IsEnabled(util.FlagDaemon) && flags.IsEnabled(util.FlagBridgeMode)
}
func (c *RemoteControlServerCommand) Execute(_ context.Context, _ []string, _ *ExecContext) (string, error) {
	return "__remote_control_server__", nil
}

// ─── /voice ─────────────────────────────────────────────────────────────────
// Gated: feature('VOICE_MODE')

type VoiceCommand struct{ BaseCommand }

func (c *VoiceCommand) Name() string        { return "voice" }
func (c *VoiceCommand) Description() string { return "Toggle voice input mode" }
func (c *VoiceCommand) Type() CommandType   { return CommandTypeInteractive }
func (c *VoiceCommand) IsEnabled(_ *ExecContext) bool {
	return util.NewFeatureFlagStore().IsEnabled(util.FlagVoiceMode)
}
func (c *VoiceCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{
		Component: "voice",
		Data:      map[string]interface{}{"args": args},
	}, nil
}

// ─── /force-snip ────────────────────────────────────────────────────────────
// Gated: feature('HISTORY_SNIP')

type ForceSnipCommand struct{ BaseCommand }

func (c *ForceSnipCommand) Name() string        { return "force-snip" }
func (c *ForceSnipCommand) Description() string { return "Force a history snip at the current point" }
func (c *ForceSnipCommand) Type() CommandType   { return CommandTypeLocal }
func (c *ForceSnipCommand) IsEnabled(_ *ExecContext) bool {
	return util.NewFeatureFlagStore().IsEnabled(util.FlagHistorySnip)
}
func (c *ForceSnipCommand) Execute(_ context.Context, _ []string, _ *ExecContext) (string, error) {
	return "__force_snip__", nil
}

// ─── /subscribe-pr ──────────────────────────────────────────────────────────
// Gated: feature('KAIROS_GITHUB_WEBHOOKS')

type SubscribePRCommand struct{ BasePromptCommand }

func (c *SubscribePRCommand) Name() string        { return "subscribe-pr" }
func (c *SubscribePRCommand) Aliases() []string   { return []string{"subscribe"} }
func (c *SubscribePRCommand) Description() string { return "Subscribe to PR webhook events" }
func (c *SubscribePRCommand) Type() CommandType   { return CommandTypePrompt }
func (c *SubscribePRCommand) IsEnabled(_ *ExecContext) bool {
	return util.NewFeatureFlagStore().IsEnabled(util.FlagKairosGithubWebhooks)
}
func (c *SubscribePRCommand) PromptContent(args []string, _ *ExecContext) (string, error) {
	target := ""
	if len(args) > 0 {
		target = strings.Join(args, " ")
	}
	return fmt.Sprintf(`## Subscribe to PR Events

%sSet up webhook subscriptions for PR events. This enables proactive
notifications when PRs are opened, updated, or reviewed.

Configure the webhook to notify this session of:
- New PR comments
- Review requests
- Status check updates
- Merge conflicts`, func() string {
		if target != "" {
			return fmt.Sprintf("Target PR: %s\n\n", target)
		}
		return ""
	}()), nil
}

// ─── /ultraplan ─────────────────────────────────────────────────────────────
// Gated: feature('ULTRAPLAN')

type UltraplanCommand struct{ BasePromptCommand }

func (c *UltraplanCommand) Name() string { return "ultraplan" }
func (c *UltraplanCommand) Description() string {
	return "Generate a detailed implementation plan using extended thinking"
}
func (c *UltraplanCommand) Type() CommandType { return CommandTypePrompt }
func (c *UltraplanCommand) IsEnabled(_ *ExecContext) bool {
	return util.NewFeatureFlagStore().IsEnabled(util.FlagUltraplan)
}
func (c *UltraplanCommand) PromptContent(args []string, _ *ExecContext) (string, error) {
	task := "the current task"
	if len(args) > 0 {
		task = strings.Join(args, " ")
	}
	return fmt.Sprintf(`## Ultra Plan

Create a comprehensive, step-by-step implementation plan for: %s

Use extended thinking to analyze:
1. Current codebase structure and patterns
2. Dependencies and potential impact
3. Risks and edge cases
4. Testing strategy
5. Implementation order

Output a detailed plan with:
- Numbered steps with clear descriptions
- File changes needed per step
- Estimated complexity per step
- Dependency graph between steps
- Potential blockers and mitigations`, task), nil
}

// ─── /torch ─────────────────────────────────────────────────────────────────
// Gated: feature('TORCH')

type TorchCommand struct{ BasePromptCommand }

func (c *TorchCommand) Name() string        { return "torch" }
func (c *TorchCommand) Description() string { return "Deep codebase exploration and analysis" }
func (c *TorchCommand) Type() CommandType   { return CommandTypePrompt }
func (c *TorchCommand) IsEnabled(_ *ExecContext) bool {
	return util.NewFeatureFlagStore().IsEnabled(util.FlagTorch)
}
func (c *TorchCommand) PromptContent(args []string, _ *ExecContext) (string, error) {
	query := "the overall architecture"
	if len(args) > 0 {
		query = strings.Join(args, " ")
	}
	return fmt.Sprintf(`## Torch — Deep Codebase Analysis

Perform a thorough exploration and analysis of: %s

Methodology:
1. Use file search tools to discover relevant source files
2. Read key files to understand the implementation
3. Trace the execution flow from entry points
4. Map dependencies and interfaces
5. Identify patterns and anti-patterns

Output a comprehensive analysis including:
- Architecture overview
- Key components and their relationships
- Data flow diagram (text-based)
- Potential improvement areas
- Technical debt assessment`, query), nil
}

// ─── /peers ─────────────────────────────────────────────────────────────────
// Gated: feature('UDS_INBOX')

type PeersCommand struct{ BaseCommand }

func (c *PeersCommand) Name() string        { return "peers" }
func (c *PeersCommand) Description() string { return "Show connected peer sessions" }
func (c *PeersCommand) Type() CommandType   { return CommandTypeInteractive }
func (c *PeersCommand) IsEnabled(_ *ExecContext) bool {
	return util.NewFeatureFlagStore().IsEnabled(util.FlagUdsInbox)
}
func (c *PeersCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{
		Component: "peers",
		Data:      map[string]interface{}{"args": args},
	}, nil
}

// ─── /fork ──────────────────────────────────────────────────────────────────
// Gated: feature('FORK_SUBAGENT')

type ForkCommand struct{ BaseCommand }

func (c *ForkCommand) Name() string        { return "fork" }
func (c *ForkCommand) Description() string { return "Fork a sub-agent to work in parallel" }
func (c *ForkCommand) Type() CommandType   { return CommandTypeInteractive }
func (c *ForkCommand) IsEnabled(_ *ExecContext) bool {
	return util.NewFeatureFlagStore().IsEnabled(util.FlagForkSubagent)
}
func (c *ForkCommand) ArgumentHint() string { return "[task description]" }
func (c *ForkCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := map[string]interface{}{}
	if len(args) > 0 {
		data["task"] = strings.Join(args, " ")
	}
	if ectx != nil {
		data["session_id"] = ectx.SessionID
	}
	return &InteractiveResult{
		Component: "fork",
		Data:      data,
	}, nil
}

// ─── /web (remote-setup) ───────────────────────────────────────────────────
// Gated: feature('CCR_REMOTE_SETUP')

type WebRemoteSetupCommand struct{ BaseCommand }

func (c *WebRemoteSetupCommand) Name() string        { return "web" }
func (c *WebRemoteSetupCommand) Description() string { return "Configure remote connection setup" }
func (c *WebRemoteSetupCommand) Type() CommandType   { return CommandTypeInteractive }
func (c *WebRemoteSetupCommand) IsEnabled(_ *ExecContext) bool {
	return util.NewFeatureFlagStore().IsEnabled(util.FlagCcrRemoteSetup)
}
func (c *WebRemoteSetupCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{
		Component: "web-remote-setup",
		Data:      map[string]interface{}{"args": args},
	}, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Register all feature-gated commands.
// ──────────────────────────────────────────────────────────────────────────────

func init() {
	defaultRegistry.Register(
		&ProactiveCommand{},
		&BriefCommand{},
		&AssistantCommand{},
		&BridgeCommand{},
		&RemoteControlServerCommand{},
		&VoiceCommand{},
		&ForceSnipCommand{},
		&SubscribePRCommand{},
		&UltraplanCommand{},
		&TorchCommand{},
		&PeersCommand{},
		&ForkCommand{},
		&WebRemoteSetupCommand{},
	)
}
