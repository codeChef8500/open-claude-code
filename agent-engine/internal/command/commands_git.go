package command

import (
	"context"
	"fmt"
	"strings"
)

// ─── /branch ─────────────────────────────────────────────────────────────────

// BranchCommand creates a branch of the current conversation.
// Aligned with claude-code-main commands/branch/index.ts (local-jsx).
type BranchCommand struct{ BaseCommand }

func (c *BranchCommand) Name() string                  { return "branch" }
func (c *BranchCommand) Description() string           { return "Create a branch of the current conversation at this point" }
func (c *BranchCommand) ArgumentHint() string          { return "[name]" }
func (c *BranchCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *BranchCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *BranchCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	name := ""
	if len(args) > 0 {
		name = strings.Join(args, " ")
	}
	return &InteractiveResult{
		Component: "branch",
		Data: map[string]interface{}{
			"name":      name,
			"sessionID": ectx.SessionID,
		},
	}, nil
}

// ─── /diff ───────────────────────────────────────────────────────────────────

// DiffCommand shows uncommitted changes and per-turn diffs.
// Aligned with claude-code-main commands/diff/index.ts (local-jsx).
type DiffCommand struct{ BaseCommand }

func (c *DiffCommand) Name() string                  { return "diff" }
func (c *DiffCommand) Description() string           { return "Show uncommitted changes and per-turn diffs" }
func (c *DiffCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DiffCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *DiffCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{
		Component: "diff",
	}, nil
}

// ─── /pr-comments ────────────────────────────────────────────────────────────

// PRCommentsCommand addresses PR review comments.
// Aligned with claude-code-main commands/pr_comments/index.ts (prompt).
type PRCommentsCommand struct{ BasePromptCommand }

func (c *PRCommentsCommand) Name() string        { return "pr-comments" }
func (c *PRCommentsCommand) Aliases() []string   { return []string{"pr_comments"} }
func (c *PRCommentsCommand) Description() string { return "Address PR review comments" }
func (c *PRCommentsCommand) ArgumentHint() string {
	return "[PR number or URL]"
}
func (c *PRCommentsCommand) Type() CommandType             { return CommandTypePrompt }
func (c *PRCommentsCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *PRCommentsCommand) PromptContent(args []string, _ *ExecContext) (string, error) {
	target := ""
	if len(args) > 0 {
		target = strings.Join(args, " ")
	}
	return fmt.Sprintf(`## Address PR Review Comments

%sPlease:
1. Read the PR review comments
2. Understand each reviewer's feedback
3. Make the requested changes
4. Respond to each comment explaining what you changed

Use available tools to read the PR comments and make code changes.`, func() string {
		if target != "" {
			return fmt.Sprintf("PR: %s\n\n", target)
		}
		return ""
	}()), nil
}

// ─── /commit-push-pr ─────────────────────────────────────────────────────────

// CommitPushPRCommand commits, pushes, and creates a PR.
// Aligned with claude-code-main commands/commit-push-pr.ts (prompt).
type CommitPushPRCommand struct{ BasePromptCommand }

func (c *CommitPushPRCommand) Name() string                  { return "commit-push-pr" }
func (c *CommitPushPRCommand) Description() string           { return "Commit changes, push, and create a pull request" }
func (c *CommitPushPRCommand) Type() CommandType             { return CommandTypePrompt }
func (c *CommitPushPRCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *CommitPushPRCommand) PromptContent(_ []string, _ *ExecContext) (string, error) {
	return `## Your Task

Commit all current changes, push to a remote branch, and create a pull request:

1. Stage all relevant changes
2. Create a commit with a descriptive message
3. Push to a new branch (or current branch if not main/master)
4. Create a pull request with a clear title and description

## Git Safety Protocol
- NEVER update the git config
- NEVER skip hooks unless explicitly requested
- NEVER force push
- Do not commit files that likely contain secrets`, nil
}

// ─── /rewind ─────────────────────────────────────────────────────────────────

// RewindCommand restores code and/or conversation to a previous point.
// Aligned with claude-code-main commands/rewind/index.ts (local).
type RewindCommand struct{ BaseCommand }

func (c *RewindCommand) Name() string                  { return "rewind" }
func (c *RewindCommand) Aliases() []string             { return []string{"checkpoint"} }
func (c *RewindCommand) Description() string           { return "Restore the code and/or conversation to a previous point" }
func (c *RewindCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *RewindCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *RewindCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{
		Component: "rewind",
	}, nil
}

func init() {
	defaultRegistry.Register(
		&BranchCommand{},
		&DiffCommand{},
		&PRCommentsCommand{},
		&CommitPushPRCommand{},
		&RewindCommand{},
	)
}
