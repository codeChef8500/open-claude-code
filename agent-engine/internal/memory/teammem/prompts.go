package teammem

import (
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/memory"
)

// ────────────────────────────────────────────────────────────────────────────
// Team memory prompts — aligned with claude-code-main src/memdir/teamMemPrompts.ts
// ────────────────────────────────────────────────────────────────────────────

// BuildCombinedMemoryPrompt builds the combined prompt when both auto memory
// and team memory are enabled. Uses a closed four-type taxonomy with per-type
// scope guidance.
func BuildCombinedMemoryPrompt(projectRoot string, extraGuidelines []string, skipIndex bool) string {
	autoDir := memory.GetAutoMemPath(projectRoot)
	teamDir := GetTeamMemPath(projectRoot)

	var howToSave []string
	if skipIndex {
		howToSave = []string{
			"## How to save memories",
			"",
			"Write each memory to its own file in the chosen directory (private or team, per the type's scope guidance) using this frontmatter format:",
			"",
		}
		howToSave = append(howToSave, memoryFrontmatterExample...)
		howToSave = append(howToSave,
			"",
			"- Keep the name, description, and type fields in memory files up-to-date with the content",
			"- Organize memory semantically by topic, not chronologically",
			"- Update or remove memories that turn out to be wrong or outdated",
			"- Do not write duplicate memories. First check if there is an existing memory you can update before writing a new one.",
		)
	} else {
		howToSave = []string{
			"## How to save memories",
			"",
			"Saving a memory is a two-step process:",
			"",
			"**Step 1** — write the memory to its own file in the chosen directory (private or team, per the type's scope guidance) using this frontmatter format:",
			"",
		}
		howToSave = append(howToSave, memoryFrontmatterExample...)
		howToSave = append(howToSave,
			"",
			fmt.Sprintf("**Step 2** — add a pointer to that file in the same directory's `%s`. Each directory (private and team) has its own `%s` index — each entry should be one line, under ~150 characters: `- [Title](file.md) — one-line hook`. They have no frontmatter. Never write memory content directly into a `%s`.",
				memory.AutoMemEntrypointName, memory.AutoMemEntrypointName, memory.AutoMemEntrypointName),
			"",
			fmt.Sprintf("- Both `%s` indexes are loaded into your conversation context — lines after %d will be truncated, so keep them concise",
				memory.AutoMemEntrypointName, memory.MaxEntrypointLines),
			"- Keep the name, description, and type fields in memory files up-to-date with the content",
			"- Organize memory semantically by topic, not chronologically",
			"- Update or remove memories that turn out to be wrong or outdated",
			"- Do not write duplicate memories. First check if there is an existing memory you can update before writing a new one.",
		)
	}

	lines := []string{
		"# Memory",
		"",
		fmt.Sprintf("You have a persistent, file-based memory system with two directories: a private directory at `%s` and a shared team directory at `%s`. %s",
			autoDir, teamDir, memory.DirsExistGuidance),
		"",
		"You should build up this memory system over time so that future conversations can have a complete picture of who the user is, how they'd like to collaborate with you, what behaviors to avoid or repeat, and the context behind the work the user gives you.",
		"",
		"If the user explicitly asks you to remember something, save it immediately as whichever type fits best. If they ask you to forget something, find and remove the relevant entry.",
		"",
		"## Memory scope",
		"",
		"There are two scope levels:",
		"",
		fmt.Sprintf("- private: memories that are private between you and the current user. They persist across conversations with only this specific user and are stored at the root `%s`.", autoDir),
		fmt.Sprintf("- team: memories that are shared with and contributed by all of the users who work within this project directory. Team memories are synced at the beginning of every session and they are stored at `%s`.", teamDir),
		"",
	}
	lines = append(lines, typesSectionCombined...)
	lines = append(lines, whatNotToSaveSection...)
	lines = append(lines,
		"- You MUST avoid saving sensitive data within shared team memories. For example, never save API keys or user credentials.",
		"",
	)
	lines = append(lines, howToSave...)
	lines = append(lines, "")
	lines = append(lines, whenToAccessTeamSection...)
	lines = append(lines, memoryDriftCaveat)
	lines = append(lines, "")
	lines = append(lines, trustingRecallSection...)
	lines = append(lines, "")
	lines = append(lines, memoryVsPersistence...)
	if len(extraGuidelines) > 0 {
		lines = append(lines, extraGuidelines...)
	}
	lines = append(lines, "")
	lines = append(lines, memory.BuildSearchingPastContextSection(autoDir)...)

	return strings.Join(lines, "\n")
}

// ── Prompt content constants (team-specific) ───────────────────────────────

var memoryFrontmatterExample = []string{
	"```yaml",
	"---",
	"name: descriptive-kebab-case-name",
	"description: One-sentence summary of what this memory contains",
	"type: user | feedback | project | reference",
	"---",
	"```",
}

var typesSectionCombined = []string{
	"## Memory types",
	"",
	"Each memory file must have a `type` field in its frontmatter. Use one of:",
	"",
	"### `user` — Who the user is",
	"<scope>private</scope>",
	"Their role, expertise, preferences, communication style.",
	"Example: \"Senior backend engineer who prefers Go and dislikes ORMs.\"",
	"",
	"### `feedback` — How to collaborate",
	"<scope>private</scope>",
	"Direct feedback on your behavior that should change future interactions.",
	"Example: \"Don't add comments unless asked — the user finds them noisy.\"",
	"",
	"### `project` — What the project is",
	"<scope>team</scope>",
	"Architecture, key decisions, conventions, and context that is NOT derivable from the code itself.",
	"Example: \"The API gateway rate-limits at 100 req/min per IP; this is an ops decision, not in code.\"",
	"",
	"### `reference` — Useful lookups",
	"<scope>team</scope>",
	"URLs, credentials locations, command cheat-sheets.",
	"Example: \"Staging dashboard: https://staging.internal.example.com/admin\"",
	"",
}

var whatNotToSaveSection = []string{
	"## What NOT to save",
	"- Anything derivable from the current project state (code patterns, directory structure, dependency versions, build commands, git history).",
	"- Transient task status (\"currently debugging X\") — use tasks/plans instead.",
	"- Secrets, tokens, or credentials — point to where they are stored, never store the value.",
}

var whenToAccessTeamSection = []string{
	"## When to access memories",
	"- When memories (personal or team) seem relevant, or the user references prior work with them or others in their organization.",
	"- You MUST access memory when the user explicitly asks you to check, recall, or remember.",
	"- If the user says to *ignore* or *not use* memory: proceed as if MEMORY.md were empty. Do not apply remembered facts, cite, compare against, or mention memory content.",
}

const memoryDriftCaveat = "- Team memories may have been written by a different user in a different context. Treat them as hints, not absolutes — verify critical details against the current project state."

var trustingRecallSection = []string{
	"## Trusting recalled memories",
	"Memory entries may be out of date. When a recalled memory conflicts with what you observe in the current project state (e.g., the code, tests, or directory layout), **trust the project state**. Update or remove the stale memory.",
}

var memoryVsPersistence = []string{
	"## Memory and other forms of persistence",
	"Memory is one of several persistence mechanisms available to you as you assist the user in a given conversation. The distinction is often that memory can be recalled in future conversations and should not be used for persisting information that is only useful within the scope of the current conversation.",
	"- When to use or update a plan instead of memory: If you are about to start a non-trivial implementation task and would like to reach alignment with the user on your approach you should use a Plan rather than saving this information to memory. Similarly, if you already have a plan within the conversation and you have changed your approach persist that change by updating the plan rather than saving a memory.",
	"- When to use or update tasks instead of memory: When you need to break your work in current conversation into discrete steps or keep track of your progress use tasks instead of saving to memory. Tasks are great for persisting information about the work that needs to be done in the current conversation, but memory should be reserved for information that will be useful in future conversations.",
}
