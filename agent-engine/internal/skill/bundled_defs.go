package skill

// InitBundledSkills registers all programmatic bundled skill definitions.
// Call once at startup before any skill discovery.
func InitBundledSkills() {
	RegisterBundledSkill(batchSkillDef)
	RegisterBundledSkill(verifySkillDef)
	RegisterBundledSkill(simplifySkillDef)
	RegisterBundledSkill(rememberSkillDef)
	RegisterBundledSkill(debugSkillDef)
}

var batchSkillDef = BundledSkillDefinition{
	Name:        "batch",
	Description: "Orchestrate large-scale parallelizable changes across a codebase",
	WhenToUse:   "When you need to apply similar changes across many files using isolated worktree agents",
	AllowedTools: []string{
		"Bash", "Read", "Write", "Edit", "MultiEdit",
		"Glob", "Grep", "Agent",
	},
	Context: ExecFork,
	Agent:   "agent",
	StaticPrompt: `You are orchestrating a large batch of parallelizable changes across a codebase.

## Phases of work

### 1. Research
- Understand the scope of changes needed
- Identify all files and patterns affected
- Determine if changes are truly parallelizable (independent of each other)

### 2. Plan
- Break the work into discrete, independent units
- Each unit should be completable by a single worker agent
- Define clear success criteria for each unit

### 3. Spawn Workers
- Use the Agent tool to spawn isolated worker agents
- Each worker gets a specific, well-defined task
- Workers operate in isolated git worktrees to avoid conflicts

### 4. Track Progress
- Monitor worker completion
- Validate results meet success criteria
- Report summary of changes made

## Important Rules
- Each worker must make independent, non-conflicting changes
- Always verify the scope before spawning workers
- Prefer smaller, focused worker tasks over large ones
`,
}

var verifySkillDef = BundledSkillDefinition{
	Name:        "verify",
	Description: "Verify recent changes by running tests and lint checks",
	WhenToUse:   "After making code changes, to confirm nothing is broken",
	AllowedTools: []string{
		"Bash", "Read", "Glob", "Grep",
	},
	StaticPrompt: `Verify that recent code changes are correct by running the project's test suite and linting tools.

## Steps
1. Identify the project's test command (look for package.json scripts, Makefile targets, go test, pytest, etc.)
2. Run the test suite
3. Run any configured linters
4. Report results clearly:
   - Number of tests passed/failed
   - Any lint errors or warnings
   - Summary of what was verified
`,
}

var simplifySkillDef = BundledSkillDefinition{
	Name:        "simplify",
	Description: "Simplify and refactor code while preserving behavior",
	WhenToUse:   "When code is overly complex and could benefit from simplification",
	AllowedTools: []string{
		"Read", "Write", "Edit", "MultiEdit", "Glob", "Grep", "Bash",
	},
	StaticPrompt: `Analyze the specified code and simplify it while preserving its behavior.

## Approach
1. Read and understand the code's purpose and behavior
2. Identify complexity sources:
   - Unnecessary abstractions
   - Duplicated logic
   - Over-engineered patterns
   - Dead code
3. Propose simplifications with clear explanations
4. Apply changes incrementally, verifying behavior is preserved
5. Run tests after each change if available

## Rules
- Never change external API or behavior
- Prefer removing code over adding code
- Each change should be independently correct
`,
}

var rememberSkillDef = BundledSkillDefinition{
	Name:        "remember",
	Description: "Review auto-memory entries and promote to permanent memory",
	WhenToUse:   "When you want to review, clean up, or promote auto-saved memories",
	DisableModelInvocation: true,
	StaticPrompt: `Review all auto-memory entries and produce a structured report.

## Steps
1. Read the auto-memory file (CLAUDE.local.md or equivalent)
2. For each entry, evaluate:
   - Is it still relevant?
   - Is it duplicated elsewhere?
   - Does it conflict with other entries?
   - Should it be promoted to shared memory (CLAUDE.md)?
3. Output a structured report:
   - Entries to keep as-is
   - Entries to promote to CLAUDE.md
   - Entries to remove (outdated/duplicate)
   - Entries with conflicts to resolve
4. Wait for user approval before making changes
`,
}

var debugSkillDef = BundledSkillDefinition{
	Name:        "debug",
	Description: "Diagnose and fix issues in the codebase",
	WhenToUse:   "When encountering errors, unexpected behavior, or test failures",
	AllowedTools: []string{
		"Bash", "Read", "Glob", "Grep",
	},
	StaticPrompt: `Diagnose the reported issue systematically.

## Approach
1. Reproduce the issue
   - Run the failing command/test to confirm the error
   - Capture exact error messages and stack traces

2. Investigate root cause
   - Read relevant source files
   - Search for related patterns in the codebase
   - Check recent changes (git log, git diff)
   - Add diagnostic logging if needed

3. Identify the fix
   - Address root cause, not symptoms
   - Prefer minimal changes
   - Consider edge cases

4. Verify the fix
   - Run the originally failing command/test
   - Ensure no regressions

## Rules
- Always reproduce before fixing
- One root cause at a time
- Log your reasoning at each step
`,
}
