package command

import (
	"fmt"
	"strings"
)

// ─── /statusline ─────────────────────────────────────────────────────────────
// Aligned with claude-code-main commands/statusline.tsx (prompt).
// Sets up the status line UI via a sub-agent.

// StatuslineCommand sets up the status line UI.
type StatuslineCommand struct{ BasePromptCommand }

func (c *StatuslineCommand) Name() string        { return "statusline" }
func (c *StatuslineCommand) Description() string { return "Set up Claude Code's status line UI" }
func (c *StatuslineCommand) Type() CommandType   { return CommandTypePrompt }
func (c *StatuslineCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *StatuslineCommand) PromptMeta() *PromptCommandMeta {
	return &PromptCommandMeta{
		ProgressMessage:       "setting up statusLine",
		AllowedTools:          []string{"Task", "Read(~/**)", "Edit(~/.claude/settings.json)"},
		DisableNonInteractive: true,
	}
}

func (c *StatuslineCommand) PromptContent(args []string, _ *ExecContext) (string, error) {
	prompt := strings.TrimSpace(strings.Join(args, " "))
	if prompt == "" {
		prompt = "Configure my statusLine from my shell PS1 configuration"
	}
	return fmt.Sprintf(`Create a Task with subagent_type "statusline-setup" and the prompt "%s"`, prompt), nil
}

func init() {
	defaultRegistry.Register(&StatuslineCommand{})
}
