package command

import (
	"context"
	"fmt"
	"runtime"
	"strings"
)

// ─── /config ─────────────────────────────────────────────────────────────────

// ConfigCommand opens the config panel.
// Aligned with claude-code-main commands/config/index.ts (local-jsx).
type ConfigCommand struct{ BaseCommand }

func (c *ConfigCommand) Name() string                  { return "config" }
func (c *ConfigCommand) Aliases() []string             { return []string{"settings"} }
func (c *ConfigCommand) Description() string           { return "Open config panel" }
func (c *ConfigCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *ConfigCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ConfigCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	// Pass current config state to the TUI component.
	data := map[string]interface{}{}
	if ectx != nil {
		data["model"] = ectx.Model
		data["permission_mode"] = ectx.PermissionMode
		data["auto_mode"] = ectx.AutoMode
		data["verbose"] = ectx.Verbose
		data["plan_mode"] = ectx.PlanModeActive
		data["fast_mode"] = ectx.FastMode
		data["effort"] = ectx.EffortLevel
	}
	if len(args) > 0 {
		data["key"] = args[0]
		if len(args) > 1 {
			data["value"] = strings.Join(args[1:], " ")
		}
	}
	return &InteractiveResult{
		Component: "config",
		Data:      data,
	}, nil
}

// ─── /mcp ────────────────────────────────────────────────────────────────────

// McpCommand manages MCP server connections.
// Aligned with claude-code-main commands/mcp/index.ts (local-jsx, immediate).
type McpCommand struct{ BaseCommand }

func (c *McpCommand) Name() string                  { return "mcp" }
func (c *McpCommand) Description() string           { return "Manage MCP servers" }
func (c *McpCommand) ArgumentHint() string          { return "[list|add|remove|restart]" }
func (c *McpCommand) IsImmediate() bool             { return true }
func (c *McpCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *McpCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *McpCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := map[string]interface{}{}
	if len(args) > 0 {
		data["subcommand"] = args[0]
		data["args"] = args[1:]
	}
	if ectx != nil {
		data["servers"] = ectx.ActiveMCPServers
	}
	return &InteractiveResult{
		Component: "mcp",
		Data:      data,
	}, nil
}

// ─── /init ───────────────────────────────────────────────────────────────────

// InitCommand initializes project configuration files.
type InitCommand struct{ BasePromptCommand }

func (c *InitCommand) Name() string { return "init" }
func (c *InitCommand) Description() string {
	return "Initialize project configuration (CLAUDE.md, .mcp.json)."
}
func (c *InitCommand) Type() CommandType             { return CommandTypePrompt }
func (c *InitCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *InitCommand) PromptContent(args []string, _ *ExecContext) (string, error) {
	return `## Your Task
Initialize the project configuration for this codebase:

1. Analyze the project structure, tech stack, and build system.
2. Create a CLAUDE.md file in the project root with:
   - Project overview and description
   - Build and test commands
   - Code style and conventions
   - Important patterns and architecture notes
3. If relevant, suggest a .mcp.json config for useful MCP integrations.

Be concise but comprehensive. Focus on information that would help an AI assistant work effectively in this codebase.`, nil
}

// ─── /review ─────────────────────────────────────────────────────────────────

// ReviewCommand triggers a code review prompt.
type ReviewCommand struct{ BasePromptCommand }

func (c *ReviewCommand) Name() string { return "review" }
func (c *ReviewCommand) Description() string {
	return "Review code changes. Usage: /review [file-or-commit]"
}
func (c *ReviewCommand) Type() CommandType             { return CommandTypePrompt }
func (c *ReviewCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ReviewCommand) PromptContent(args []string, _ *ExecContext) (string, error) {
	target := "staged and unstaged changes"
	if len(args) > 0 {
		target = strings.Join(args, " ")
	}
	return fmt.Sprintf(`## Code Review

Review %s focusing on:

1. **Correctness**: Logic errors, edge cases, null/nil checks
2. **Security**: Injection, secrets exposure, unsafe operations
3. **Performance**: Unnecessary allocations, N+1 queries, missing indexes
4. **Style**: Consistency, naming, idiomatic patterns
5. **Tests**: Missing coverage, brittle assertions

First examine the changes, then provide a structured review with severity ratings (critical/warning/suggestion) for each finding.`, target), nil
}

// ─── /commit ─────────────────────────────────────────────────────────────────

// CommitCommand creates a git commit with an auto-generated message.
type CommitCommand struct{ BasePromptCommand }

func (c *CommitCommand) Name() string { return "commit" }
func (c *CommitCommand) Description() string {
	return "Create a git commit with an auto-generated message."
}
func (c *CommitCommand) Type() CommandType             { return CommandTypePrompt }
func (c *CommitCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *CommitCommand) PromptContent(_ []string, _ *ExecContext) (string, error) {
	return `## Context

- Current git status: !` + "`git status`" + `
- Current git diff (staged and unstaged changes): !` + "`git diff HEAD`" + `
- Current branch: !` + "`git branch --show-current`" + `
- Recent commits: !` + "`git log --oneline -10`" + `

## Git Safety Protocol

- NEVER update the git config
- NEVER skip hooks (--no-verify, --no-gpg-sign, etc) unless the user explicitly requests it
- CRITICAL: ALWAYS create NEW commits. NEVER use git commit --amend, unless the user explicitly requests it
- Do not commit files that likely contain secrets (.env, credentials.json, etc)
- If there are no changes to commit, do not create an empty commit

## Your Task

Based on the above changes, create a single git commit:

1. Analyze all staged changes and draft a commit message:
   - Look at the recent commits to follow this repository's commit message style
   - Summarize the nature of the changes (new feature, enhancement, bug fix, refactoring, test, docs, etc.)
   - Draft a concise (1-2 sentences) commit message that focuses on the "why" rather than the "what"

2. Stage relevant files and create the commit.`, nil
}

// ─── /version ────────────────────────────────────────────────────────────────

// VersionCommand shows the agent engine version.
type VersionCommand struct{ BaseCommand }

func (c *VersionCommand) Name() string                  { return "version" }
func (c *VersionCommand) Aliases() []string             { return []string{"v"} }
func (c *VersionCommand) Description() string           { return "Show agent engine version information." }
func (c *VersionCommand) Type() CommandType             { return CommandTypeLocal }
func (c *VersionCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *VersionCommand) Execute(_ context.Context, _ []string, _ *ExecContext) (string, error) {
	return fmt.Sprintf("Agent Engine v0.1.0\nGo %s\nOS/Arch: %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH), nil
}

// ─── /doctor ─────────────────────────────────────────────────────────────────

// DoctorCommand runs diagnostics on the agent engine setup.
type DoctorCommand struct{ BaseCommand }

func (c *DoctorCommand) Name() string { return "doctor" }
func (c *DoctorCommand) Description() string {
	return "Run diagnostics to check setup and connectivity."
}
func (c *DoctorCommand) Type() CommandType             { return CommandTypeLocal }
func (c *DoctorCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *DoctorCommand) Execute(_ context.Context, _ []string, ectx *ExecContext) (string, error) {
	var checks []string
	checks = append(checks, "Diagnostics:")
	checks = append(checks, fmt.Sprintf("  ✓ Go runtime: %s", runtime.Version()))
	checks = append(checks, fmt.Sprintf("  ✓ OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	checks = append(checks, fmt.Sprintf("  ✓ NumCPU: %d", runtime.NumCPU()))

	if ectx != nil {
		checks = append(checks, fmt.Sprintf("  ✓ Session: %s", ectx.SessionID))
		checks = append(checks, fmt.Sprintf("  ✓ WorkDir: %s", ectx.WorkDir))
		checks = append(checks, fmt.Sprintf("  ✓ Model: %s", ectx.Model))
		if len(ectx.ActiveMCPServers) > 0 {
			checks = append(checks, fmt.Sprintf("  ✓ MCP servers: %d connected", len(ectx.ActiveMCPServers)))
		} else {
			checks = append(checks, "  - MCP servers: none connected")
		}
	} else {
		checks = append(checks, "  - No active session")
	}

	return strings.Join(checks, "\n"), nil
}

// ─── /bug-report ─────────────────────────────────────────────────────────────

// BugReportCommand generates a bug report template with system info.
type BugReportCommand struct{ BaseCommand }

func (c *BugReportCommand) Name() string                  { return "bug-report" }
func (c *BugReportCommand) Aliases() []string             { return []string{"bugreport"} }
func (c *BugReportCommand) Description() string           { return "Generate a bug report with system info." }
func (c *BugReportCommand) Type() CommandType             { return CommandTypeLocal }
func (c *BugReportCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *BugReportCommand) Execute(_ context.Context, _ []string, ectx *ExecContext) (string, error) {
	var sb strings.Builder
	sb.WriteString("## Bug Report\n\n")
	sb.WriteString("### System Information\n")
	sb.WriteString("- Agent Engine: v0.1.0\n")
	sb.WriteString(fmt.Sprintf("- Go: %s\n", runtime.Version()))
	sb.WriteString(fmt.Sprintf("- OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH))
	if ectx != nil {
		sb.WriteString(fmt.Sprintf("- Session: %s\n", ectx.SessionID))
		sb.WriteString(fmt.Sprintf("- Model: %s\n", ectx.Model))
		sb.WriteString(fmt.Sprintf("- Turns: %d\n", ectx.TurnCount))
		sb.WriteString(fmt.Sprintf("- Total tokens: %d\n", ectx.TotalTokens))
		sb.WriteString(fmt.Sprintf("- MCP servers: %d\n", len(ectx.ActiveMCPServers)))
	}
	sb.WriteString("\n### Steps to Reproduce\n\n1. \n2. \n3. \n")
	sb.WriteString("\n### Expected Behavior\n\n\n")
	sb.WriteString("\n### Actual Behavior\n\n\n")
	return sb.String(), nil
}

// ─── /quit ───────────────────────────────────────────────────────────────────

// QuitCommand exits the session.
type QuitCommand struct{ BaseCommand }

func (c *QuitCommand) Name() string                  { return "quit" }
func (c *QuitCommand) Aliases() []string             { return []string{"q", "exit"} }
func (c *QuitCommand) Description() string           { return "Exit the current session" }
func (c *QuitCommand) IsImmediate() bool             { return true }
func (c *QuitCommand) Type() CommandType             { return CommandTypeLocal }
func (c *QuitCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *QuitCommand) Execute(_ context.Context, _ []string, _ *ExecContext) (string, error) {
	return "__quit__", nil
}

// ─── Register Phase 12 commands ──────────────────────────────────────────────

func init() {
	defaultRegistry.Register(
		&ConfigCommand{},
		&McpCommand{},
		&InitCommand{},
		&ReviewCommand{},
		&CommitCommand{},
		&VersionCommand{},
		&DoctorCommand{},
		&BugReportCommand{},
		&QuitCommand{},
	)
}
