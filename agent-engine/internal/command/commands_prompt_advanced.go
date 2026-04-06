package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/util"
)

// ──────────────────────────────────────────────────────────────────────────────
// Advanced prompt command implementations.
// These replace the basic stubs in commands_phase12.go with full logic
// aligned with claude-code-main source.
// ──────────────────────────────────────────────────────────────────────────────

// ─── /commit deep implementation ────────────────────────────────────────────

// commitPromptContent generates the full /commit prompt.
// Aligned with claude-code-main commands/commit.ts.
func commitPromptContent(args []string, ectx *ExecContext) (string, error) {
	prompt := `## Context

- Current git status:
` + "```\n!`git status`\n```" + `

- Current git diff (staged and unstaged changes):
` + "```\n!`git diff HEAD`\n```" + `

- Current branch:
` + "```\n!`git branch --show-current`\n```" + `

- Recent commits:
` + "```\n!`git log --oneline -10`\n```" + `

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

2. Stage relevant files and create the commit.`

	// Execute shell commands in prompt if services available.
	if ectx != nil && ectx.Services != nil && ectx.Services.Shell != nil {
		var err error
		prompt, err = ExecuteShellCommandsInPrompt(
			context.Background(), prompt, ectx.WorkDir, ectx.Services.Shell, 30,
		)
		if err != nil {
			// Continue with unresolved shell commands — the model can still work.
			_ = err
		}
	}

	return prompt, nil
}

// commitPromptMeta returns metadata for the /commit command.
func commitPromptMeta() *PromptCommandMeta {
	return &PromptCommandMeta{
		ProgressMessage: "creating commit",
		AllowedTools: []string{
			"Bash(git add:*)",
			"Bash(git status:*)",
			"Bash(git diff:*)",
			"Bash(git commit:*)",
			"Bash(git log:*)",
		},
	}
}

// ─── /review deep implementation ────────────────────────────────────────────

// reviewPromptContent generates the full /review prompt.
// Aligned with claude-code-main commands/review.ts.
func reviewPromptContent(args []string, ectx *ExecContext) (string, error) {
	prompt := `## Code Review

You are reviewing local code changes. Here is the context:

- Git status:
` + "```\n!`git status`\n```" + `

- Changes to review:
` + "```\n!`git diff`\n```" + `

- Staged changes:
` + "```\n!`git diff --cached`\n```" + `

- Recent commits for context:
` + "```\n!`git log --oneline -5`\n```" + `

## Review Instructions

Please review the code changes above, focusing on:

1. **Correctness**: Logic errors, edge cases, null/nil checks, off-by-one errors
2. **Security**: Injection vulnerabilities, secrets exposure, unsafe operations, auth bypasses
3. **Performance**: Unnecessary allocations, N+1 queries, missing indexes, inefficient algorithms
4. **Style**: Consistency with codebase, naming conventions, idiomatic patterns
5. **Tests**: Missing test coverage, brittle assertions, untested edge cases
6. **Documentation**: Missing or outdated comments, unclear variable names

For each finding, provide:
- **File and line number**
- **Severity**: critical / warning / suggestion
- **Description**: What the issue is
- **Recommendation**: How to fix it

Start with a brief summary of the overall changes, then list findings by severity (critical first).`

	if len(args) > 0 {
		prompt = strings.Replace(prompt,
			"!`git diff`",
			fmt.Sprintf("!`git diff %s`", strings.Join(args, " ")),
			1,
		)
	}

	// Execute shell commands in prompt.
	if ectx != nil && ectx.Services != nil && ectx.Services.Shell != nil {
		prompt, _ = ExecuteShellCommandsInPrompt(
			context.Background(), prompt, ectx.WorkDir, ectx.Services.Shell, 30,
		)
	}

	return prompt, nil
}

// reviewPromptMeta returns metadata for the /review command.
func reviewPromptMeta() *PromptCommandMeta {
	return &PromptCommandMeta{
		ProgressMessage: "reviewing code changes",
		AllowedTools: []string{
			"Bash(git diff:*)",
			"Bash(git status:*)",
			"Bash(git log:*)",
			"Bash(git show:*)",
			"Read",
			"Glob",
			"Grep",
			"LS",
		},
	}
}

// ─── /security-review ───────────────────────────────────────────────────────

// SecurityReviewCommand performs a security-focused code review.
// Aligned with claude-code-main commands/security-review.ts.
type SecurityReviewCommand struct{ BasePromptCommand }

func (c *SecurityReviewCommand) Name() string        { return "security-review" }
func (c *SecurityReviewCommand) Description() string { return "Complete a security review of the pending changes on the current branch" }
func (c *SecurityReviewCommand) Type() CommandType   { return CommandTypePrompt }
func (c *SecurityReviewCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *SecurityReviewCommand) PromptMeta() *PromptCommandMeta {
	return &PromptCommandMeta{
		ProgressMessage: "analyzing code changes for security risks",
		AllowedTools: []string{
			"Bash(git diff:*)",
			"Bash(git status:*)",
			"Bash(git log:*)",
			"Bash(git show:*)",
			"Bash(git remote show:*)",
			"Read",
			"Glob",
			"Grep",
			"LS",
			"Task",
		},
	}
}

func (c *SecurityReviewCommand) PromptContent(_ []string, ectx *ExecContext) (string, error) {
	prompt := securityReviewPrompt

	// Execute shell commands in prompt.
	if ectx != nil && ectx.Services != nil && ectx.Services.Shell != nil {
		prompt, _ = ExecuteShellCommandsInPrompt(
			context.Background(), prompt, ectx.WorkDir, ectx.Services.Shell, 60,
		)
	}

	return prompt, nil
}

// securityReviewPrompt is the full security review prompt text.
// Aligned with claude-code-main commands/security-review.ts SECURITY_REVIEW_MARKDOWN.
const securityReviewPrompt = `You are a senior security engineer conducting a focused security review of the changes on this branch.

GIT STATUS:

` + "```\n!`git status`\n```" + `

FILES MODIFIED:

` + "```\n!`git diff --name-only origin/HEAD...`\n```" + `

COMMITS:

` + "```\n!`git log --no-decorate origin/HEAD...`\n```" + `

DIFF CONTENT:

` + "```\n!`git diff origin/HEAD...`\n```" + `

Review the complete diff above. This contains all code changes in the PR.

OBJECTIVE:
Perform a security-focused code review to identify HIGH-CONFIDENCE security vulnerabilities that could have real exploitation potential. This is not a general code review - focus ONLY on security implications newly added by this PR. Do not comment on existing security concerns.

CRITICAL INSTRUCTIONS:
1. MINIMIZE FALSE POSITIVES: Only flag issues where you're >80% confident of actual exploitability
2. AVOID NOISE: Skip theoretical issues, style concerns, or low-impact findings
3. FOCUS ON IMPACT: Prioritize vulnerabilities that could lead to unauthorized access, data breaches, or system compromise
4. EXCLUSIONS: Do NOT report the following issue types:
   - Denial of Service (DOS) vulnerabilities, even if they allow service disruption
   - Secrets or sensitive data stored on disk (these are handled by other processes)
   - Rate limiting or resource exhaustion issues

SECURITY CATEGORIES TO EXAMINE:

**Input Validation Vulnerabilities:**
- SQL injection via unsanitized user input
- Command injection in system calls or subprocesses
- XXE injection in XML parsing
- Template injection in templating engines
- NoSQL injection in database queries
- Path traversal in file operations

**Authentication & Authorization Issues:**
- Authentication bypass logic
- Privilege escalation paths
- Session management flaws
- JWT token vulnerabilities
- Authorization logic bypasses

**Crypto & Secrets Management:**
- Hardcoded API keys, passwords, or tokens
- Weak cryptographic algorithms or implementations
- Improper key storage or management
- Cryptographic randomness issues
- Certificate validation bypasses

**Injection & Code Execution:**
- Remote code execution via deserialization
- YAML deserialization vulnerabilities
- Eval injection in dynamic code execution
- XSS vulnerabilities in web applications (reflected, stored, DOM-based)

**Data Exposure:**
- Sensitive data logging or storage
- PII handling violations
- API endpoint data leakage
- Debug information exposure

ANALYSIS METHODOLOGY:

Phase 1 - Repository Context Research (Use file search tools):
- Identify existing security frameworks and libraries in use
- Look for established secure coding patterns in the codebase
- Examine existing sanitization and validation patterns
- Understand the project's security model and threat model

Phase 2 - Comparative Analysis:
- Compare new code changes against existing security patterns
- Identify deviations from established secure practices
- Look for inconsistent security implementations
- Flag code that introduces new attack surfaces

Phase 3 - Vulnerability Assessment:
- Examine each modified file for security implications
- Trace data flow from user inputs to sensitive operations
- Look for privilege boundaries being crossed unsafely
- Identify injection points and unsafe deserialization

REQUIRED OUTPUT FORMAT:

You MUST output your findings in markdown. The markdown output should contain the file, line number, severity, category, description, exploit scenario, and fix recommendation.

SEVERITY GUIDELINES:
- **HIGH**: Directly exploitable vulnerabilities leading to RCE, data breach, or authentication bypass
- **MEDIUM**: Vulnerabilities requiring specific conditions but with significant impact
- **LOW**: Defense-in-depth issues or lower-impact vulnerabilities

CONFIDENCE SCORING:
- 0.9-1.0: Certain exploit path identified
- 0.8-0.9: Clear vulnerability pattern with known exploitation methods
- 0.7-0.8: Suspicious pattern requiring specific conditions to exploit
- Below 0.7: Don't report (too speculative)

FINAL REMINDER:
Focus on HIGH and MEDIUM findings only. Better to miss some theoretical issues than flood the report with false positives.

START ANALYSIS:

Begin your analysis now. Do this in 3 steps:

1. Use a sub-task to identify vulnerabilities. Use the repository exploration tools to understand the codebase context, then analyze the PR changes for security implications.
2. Then for each vulnerability identified, create a new sub-task to filter out false-positives.
3. Filter out any vulnerabilities where the sub-task reported a confidence less than 8.

Your final reply must contain the markdown report and nothing else.`

// ─── /ultrareview ───────────────────────────────────────────────────────────

// UltrareviewCommand triggers remote bug hunting review.
// Aligned with claude-code-main commands/review.ts ultrareview.
type UltrareviewCommand struct{ BaseCommand }

func (c *UltrareviewCommand) Name() string        { return "ultrareview" }
func (c *UltrareviewCommand) Description() string { return "Launch remote bug hunting review" }
func (c *UltrareviewCommand) Type() CommandType   { return CommandTypeInteractive }
func (c *UltrareviewCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *UltrareviewCommand) IsHidden() bool { return true }
func (c *UltrareviewCommand) ExecuteInteractive(_ context.Context, args []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{
		Component: "ultrareview",
		Data:      map[string]interface{}{"args": args},
	}, nil
}

// ─── /init deep implementation ──────────────────────────────────────────────

// initPromptContentOld is the simple init prompt.
const initPromptContentOld = `## Your Task
Initialize the project configuration for this codebase:

1. Analyze the project structure, tech stack, and build system.
2. Create a CLAUDE.md file in the project root with:
   - Project overview and description
   - Build and test commands
   - Code style and conventions
   - Important patterns and architecture notes
3. If relevant, suggest a .mcp.json config for useful MCP integrations.

Be concise but comprehensive. Focus on information that would help an AI assistant work effectively in this codebase.`

// initPromptContentNew is the comprehensive 8-phase init prompt.
// Aligned with claude-code-main commands/init.ts NEW_INIT_PROMPT.
const initPromptContentNew = `## Your Task: Project Initialization

You are initializing a new project workspace. Follow these phases carefully:

### Phase 1: Project Discovery
- Explore the project structure using LS, Glob, and Read tools
- Identify the programming language(s), framework(s), and build system
- Check for existing configuration files (package.json, go.mod, Cargo.toml, etc.)
- Read any existing README.md or documentation

### Phase 2: CLAUDE.md Generation
Create a CLAUDE.md file in the project root with these sections:

**Project Overview**: Brief description of what this project does

**Build & Test Commands**: 
- How to build the project
- How to run tests (all tests, single test, test with coverage)
- How to lint / format code
- How to run the dev server (if applicable)

**Code Style Guidelines**:
- Import ordering conventions
- Naming conventions (files, functions, variables, types)
- Error handling patterns
- Logging conventions

**Architecture Notes**:
- Key directories and their purposes
- Important patterns (dependency injection, event-driven, etc.)
- Database/storage patterns
- API patterns

### Phase 3: Dependency Review
- Check for outdated or vulnerable dependencies
- Note any unusual dependency choices

### Phase 4: Environment Setup
- Check for .env.example or similar environment templates
- Note required environment variables
- Check for Docker/container setup

### Phase 5: Git Configuration
- Check .gitignore completeness
- Note branch naming conventions from recent commits
- Check for CI/CD configuration

### Phase 6: MCP Integration (if applicable)
If the project would benefit from MCP integrations, create a .mcp.json with:
- Relevant MCP servers for the tech stack
- Appropriate tool configurations

### Phase 7: Skills Setup
If the project has common workflows, create skill files in .claude/commands/:
- Build and deploy skills
- Testing skills
- Database migration skills

### Phase 8: Summary
Provide a brief summary of:
- What was created/modified
- Key findings about the project
- Recommended next steps

Be thorough but concise. Focus on practical information.`

// initPromptContent returns the appropriate init prompt based on feature flags.
func initPromptContent(args []string, ectx *ExecContext) (string, error) {
	// Check for NEW_INIT feature flag.
	flags := util.NewFeatureFlagStore()
	if flags.IsEnabled(util.FlagNewInit) {
		return initPromptContentNew, nil
	}
	return initPromptContentOld, nil
}

// initPromptMeta returns metadata for the /init command.
func initPromptMeta() *PromptCommandMeta {
	return &PromptCommandMeta{
		ProgressMessage: "initializing project",
		AllowedTools: []string{
			"Read", "Write", "Edit", "MultiEdit",
			"Glob", "Grep", "LS",
			"Bash(ls:*)", "Bash(cat:*)", "Bash(find:*)",
		},
	}
}

// ─── /commit-push-pr deep implementation ────────────────────────────────────

// commitPushPrPromptContent generates the full /commit-push-pr prompt.
// Aligned with claude-code-main commands/commit-push-pr.ts.
func commitPushPrPromptContent(args []string, ectx *ExecContext) (string, error) {
	prompt := `## Context

- Current git status:
` + "```\n!`git status`\n```" + `

- Current git diff:
` + "```\n!`git diff HEAD`\n```" + `

- Current branch:
` + "```\n!`git branch --show-current`\n```" + `

- Recent commits on this branch:
` + "```\n!`git log --oneline -10`\n```" + `

- Remote tracking info:
` + "```\n!`git remote -v`\n```" + `

## Git Safety Protocol

- NEVER update the git config
- NEVER skip hooks (--no-verify, --no-gpg-sign, etc) unless the user explicitly requests it
- CRITICAL: ALWAYS create NEW commits. NEVER use git commit --amend, unless the user explicitly requests it
- Do not commit files that likely contain secrets (.env, credentials.json, etc)
- If there are no changes to commit, do not create an empty commit

## Your Task

Complete the full commit-push-PR workflow:

1. **Stage & Commit**: Stage relevant files and create a commit with a descriptive message
   - Follow the repository's commit message conventions
   - Focus on "why" rather than "what"

2. **Push**: Push the branch to the remote
   - If no upstream is set, set it with ` + "`git push -u origin <branch>`" + `

3. **Create PR**: Create a pull request using ` + "`gh pr create`" + ` (if gh CLI is available)
   - Title: derived from the commit message
   - Body: summary of changes
   - If ` + "`gh`" + ` is not available, provide the URL to create a PR manually`

	// Execute shell commands.
	if ectx != nil && ectx.Services != nil && ectx.Services.Shell != nil {
		prompt, _ = ExecuteShellCommandsInPrompt(
			context.Background(), prompt, ectx.WorkDir, ectx.Services.Shell, 30,
		)
	}

	return prompt, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Wire the deep implementations into existing commands.
// We replace the PromptContent methods on CommitCommand, ReviewCommand,
// and InitCommand at init time via the registry.
// ──────────────────────────────────────────────────────────────────────────────

// DeepCommitCommand replaces the basic CommitCommand with full logic.
type DeepCommitCommand struct{ BasePromptCommand }

func (c *DeepCommitCommand) Name() string                  { return "commit" }
func (c *DeepCommitCommand) Description() string           { return "Create a git commit with an auto-generated message." }
func (c *DeepCommitCommand) Type() CommandType             { return CommandTypePrompt }
func (c *DeepCommitCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *DeepCommitCommand) PromptContent(args []string, ectx *ExecContext) (string, error) {
	return commitPromptContent(args, ectx)
}
func (c *DeepCommitCommand) PromptMeta() *PromptCommandMeta { return commitPromptMeta() }

// DeepReviewCommand replaces the basic ReviewCommand with full logic.
type DeepReviewCommand struct{ BasePromptCommand }

func (c *DeepReviewCommand) Name() string                  { return "review" }
func (c *DeepReviewCommand) Description() string           { return "Review code changes. Usage: /review [file-or-commit]" }
func (c *DeepReviewCommand) Type() CommandType             { return CommandTypePrompt }
func (c *DeepReviewCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *DeepReviewCommand) PromptContent(args []string, ectx *ExecContext) (string, error) {
	return reviewPromptContent(args, ectx)
}
func (c *DeepReviewCommand) PromptMeta() *PromptCommandMeta { return reviewPromptMeta() }

// DeepInitCommand replaces the basic InitCommand with full logic.
type DeepInitCommand struct{ BasePromptCommand }

func (c *DeepInitCommand) Name() string                  { return "init" }
func (c *DeepInitCommand) Description() string           { return "Initialize project configuration (CLAUDE.md, skills, hooks)." }
func (c *DeepInitCommand) Type() CommandType             { return CommandTypePrompt }
func (c *DeepInitCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *DeepInitCommand) PromptContent(args []string, ectx *ExecContext) (string, error) {
	return initPromptContent(args, ectx)
}
func (c *DeepInitCommand) PromptMeta() *PromptCommandMeta { return initPromptMeta() }

// DeepCommitPushPrCommand replaces the basic commit-push-pr with full logic.
type DeepCommitPushPrCommand struct{ BasePromptCommand }

func (c *DeepCommitPushPrCommand) Name() string        { return "commit-push-pr" }
func (c *DeepCommitPushPrCommand) Aliases() []string   { return []string{"cpp"} }
func (c *DeepCommitPushPrCommand) Description() string { return "Commit changes, push to remote, and create a PR." }
func (c *DeepCommitPushPrCommand) Type() CommandType   { return CommandTypePrompt }
func (c *DeepCommitPushPrCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *DeepCommitPushPrCommand) PromptContent(args []string, ectx *ExecContext) (string, error) {
	return commitPushPrPromptContent(args, ectx)
}
func (c *DeepCommitPushPrCommand) PromptMeta() *PromptCommandMeta {
	return &PromptCommandMeta{
		ProgressMessage: "committing, pushing, and creating PR",
		AllowedTools: []string{
			"Bash(git add:*)", "Bash(git status:*)", "Bash(git diff:*)",
			"Bash(git commit:*)", "Bash(git push:*)", "Bash(git log:*)",
			"Bash(git remote:*)", "Bash(git branch:*)",
			"Bash(gh pr create:*)", "Bash(gh pr:*)",
		},
	}
}

// init registers the deep implementations, replacing basic stubs.
func init() {
	defaultRegistry.RegisterOrReplace(
		&DeepCommitCommand{},
		&DeepReviewCommand{},
		&DeepInitCommand{},
		&DeepCommitPushPrCommand{},
		&SecurityReviewCommand{},
		&UltrareviewCommand{},
	)
}
