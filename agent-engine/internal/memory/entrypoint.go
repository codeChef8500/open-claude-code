package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ────────────────────────────────────────────────────────────────────────────
// Auto-memory entrypoint management — aligned with claude-code-main
// src/memdir/memdir.ts
// ────────────────────────────────────────────────────────────────────────────

const (
	// MaxEntrypointLines is the maximum number of lines in MEMORY.md.
	MaxEntrypointLines = 200
	// MaxEntrypointBytes is the maximum byte size of MEMORY.md (~25KB).
	MaxEntrypointBytes = 25_000
	// AutoMemDisplayName is the display name for auto memory.
	AutoMemDisplayName = "auto memory"
	// DirExistsGuidance is appended to prompt lines to prevent model from
	// running mkdir before writing.
	DirExistsGuidance = "This directory already exists — write to it directly with the Write tool (do not run mkdir or check for its existence)."
	// DirsExistGuidance is the plural variant.
	DirsExistGuidance = "Both directories already exist — write to them directly with the Write tool (do not run mkdir or check for their existence)."
)

// EntrypointTruncation holds the result of truncating MEMORY.md content.
type EntrypointTruncation struct {
	Content          string
	LineCount        int
	ByteCount        int
	WasLineTruncated bool
	WasByteTruncated bool
}

// TruncateEntrypointContent truncates MEMORY.md content to the line AND byte
// caps, appending a warning naming which cap fired. Line-truncates first
// (natural boundary), then byte-truncates at the last newline before the cap.
func TruncateEntrypointContent(raw string) EntrypointTruncation {
	trimmed := strings.TrimSpace(raw)
	contentLines := strings.Split(trimmed, "\n")
	lineCount := len(contentLines)
	byteCount := len(trimmed)

	wasLineTruncated := lineCount > MaxEntrypointLines
	wasByteTruncated := byteCount > MaxEntrypointBytes

	if !wasLineTruncated && !wasByteTruncated {
		return EntrypointTruncation{
			Content:          trimmed,
			LineCount:        lineCount,
			ByteCount:        byteCount,
			WasLineTruncated: false,
			WasByteTruncated: false,
		}
	}

	truncated := trimmed
	if wasLineTruncated {
		truncated = strings.Join(contentLines[:MaxEntrypointLines], "\n")
	}

	if len(truncated) > MaxEntrypointBytes {
		cutAt := strings.LastIndex(truncated[:MaxEntrypointBytes], "\n")
		if cutAt > 0 {
			truncated = truncated[:cutAt]
		} else {
			truncated = truncated[:MaxEntrypointBytes]
		}
	}

	var reason string
	switch {
	case wasByteTruncated && !wasLineTruncated:
		reason = fmt.Sprintf("%s (limit: %s) — index entries are too long",
			formatFileSize(byteCount), formatFileSize(MaxEntrypointBytes))
	case wasLineTruncated && !wasByteTruncated:
		reason = fmt.Sprintf("%d lines (limit: %d)", lineCount, MaxEntrypointLines)
	default:
		reason = fmt.Sprintf("%d lines and %s", lineCount, formatFileSize(byteCount))
	}

	truncated += fmt.Sprintf("\n\n> WARNING: %s is %s. Only part of it was loaded. Keep index entries to one line under ~200 chars; move detail into topic files.",
		AutoMemEntrypointName, reason)

	return EntrypointTruncation{
		Content:          truncated,
		LineCount:        lineCount,
		ByteCount:        byteCount,
		WasLineTruncated: wasLineTruncated,
		WasByteTruncated: wasByteTruncated,
	}
}

// EnsureMemoryDirExists creates the memory directory if it does not exist.
func EnsureMemoryDirExists(memoryDir string) error {
	return os.MkdirAll(memoryDir, 0o700)
}

// ReadEntrypoint reads the MEMORY.md file from the auto-memory directory.
// Returns empty string if the file does not exist.
func ReadEntrypoint(projectRoot string) string {
	ep := GetAutoMemEntrypoint(projectRoot)
	data, err := os.ReadFile(ep)
	if err != nil {
		return ""
	}
	return string(data)
}

// WriteEntrypoint writes content to the MEMORY.md file.
func WriteEntrypoint(projectRoot, content string) error {
	ep := GetAutoMemEntrypoint(projectRoot)
	dir := filepath.Dir(ep)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(ep, []byte(content), 0o600)
}

// ── Memory prompt building ─────────────────────────────────────────────────

// BuildMemoryLines builds the typed-memory behavioral instructions.
// Returns the prompt lines for auto-memory.
func BuildMemoryLines(displayName, memoryDir string, extraGuidelines []string, skipIndex bool) []string {
	var howToSave []string
	if skipIndex {
		howToSave = []string{
			"## How to save memories",
			"",
			"Write each memory to its own file (e.g., `user_role.md`, `feedback_testing.md`) using this frontmatter format:",
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
			"**Step 1** — write the memory to its own file (e.g., `user_role.md`, `feedback_testing.md`) using this frontmatter format:",
			"",
		}
		howToSave = append(howToSave, memoryFrontmatterExample...)
		howToSave = append(howToSave,
			"",
			fmt.Sprintf("**Step 2** — add a pointer to that file in `%s`. `%s` is an index, not a memory — each entry should be one line, under ~150 characters: `- [Title](file.md) — one-line hook`. It has no frontmatter. Never write memory content directly into `%s`.",
				AutoMemEntrypointName, AutoMemEntrypointName, AutoMemEntrypointName),
			"",
			fmt.Sprintf("- `%s` is always loaded into your conversation context — lines after %d will be truncated, so keep the index concise",
				AutoMemEntrypointName, MaxEntrypointLines),
			"- Keep the name, description, and type fields in memory files up-to-date with the content",
			"- Organize memory semantically by topic, not chronologically",
			"- Update or remove memories that turn out to be wrong or outdated",
			"- Do not write duplicate memories. First check if there is an existing memory you can update before writing a new one.",
		)
	}

	lines := []string{
		fmt.Sprintf("# %s", displayName),
		"",
		fmt.Sprintf("You have a persistent, file-based memory system at `%s`. %s", memoryDir, DirExistsGuidance),
		"",
		"You should build up this memory system over time so that future conversations can have a complete picture of who the user is, how they'd like to collaborate with you, what behaviors to avoid or repeat, and the context behind the work the user gives you.",
		"",
		"If the user explicitly asks you to remember something, save it immediately as whichever type fits best. If they ask you to forget something, find and remove the relevant entry.",
		"",
	}
	lines = append(lines, typeSectionIndividual...)
	lines = append(lines, whatNotToSaveSection...)
	lines = append(lines, "")
	lines = append(lines, howToSave...)
	lines = append(lines, "")
	lines = append(lines, whenToAccessSection...)
	lines = append(lines, "")
	lines = append(lines, trustingRecallSection...)
	lines = append(lines, "")
	lines = append(lines, memoryVsPersistence...)
	lines = append(lines, "")

	if len(extraGuidelines) > 0 {
		lines = append(lines, extraGuidelines...)
		lines = append(lines, "")
	}

	lines = append(lines, BuildSearchingPastContextSection(memoryDir)...)

	return lines
}

// BuildMemoryPrompt builds the complete memory prompt including MEMORY.md content.
func BuildMemoryPrompt(displayName, memoryDir string, extraGuidelines []string) string {
	lines := BuildMemoryLines(displayName, memoryDir, extraGuidelines, false)

	// Read existing memory entrypoint
	ep := filepath.Join(memoryDir, AutoMemEntrypointName)
	data, err := os.ReadFile(ep)
	entrypointContent := ""
	if err == nil {
		entrypointContent = string(data)
	}

	if strings.TrimSpace(entrypointContent) != "" {
		t := TruncateEntrypointContent(entrypointContent)
		lines = append(lines, fmt.Sprintf("## %s", AutoMemEntrypointName), "", t.Content)
	} else {
		lines = append(lines,
			fmt.Sprintf("## %s", AutoMemEntrypointName),
			"",
			fmt.Sprintf("Your %s is currently empty. When you save new memories, they will appear here.", AutoMemEntrypointName),
		)
	}

	return strings.Join(lines, "\n")
}

// BuildAssistantDailyLogPrompt builds the KAIROS daily-log mode prompt.
func BuildAssistantDailyLogPrompt(memoryDir string, skipIndex bool) string {
	logPathPattern := filepath.Join(memoryDir, "logs", "YYYY", "MM", "YYYY-MM-DD.md")

	lines := []string{
		"# auto memory",
		"",
		fmt.Sprintf("You have a persistent, file-based memory system found at: `%s`", memoryDir),
		"",
		"This session is long-lived. As you work, record anything worth remembering by **appending** to today's daily log file:",
		"",
		fmt.Sprintf("`%s`", logPathPattern),
		"",
		"Substitute today's date (from `currentDate` in your context) for `YYYY-MM-DD`. When the date rolls over mid-session, start appending to the new day's file.",
		"",
		"Write each entry as a short timestamped bullet. Create the file (and parent directories) on first write if it does not exist. Do not rewrite or reorganize the log — it is append-only. A separate nightly process distills these logs into `MEMORY.md` and topic files.",
		"",
		"## What to log",
		"- User corrections and preferences (\"use bun, not npm\"; \"stop summarizing diffs\")",
		"- Facts about the user, their role, or their goals",
		"- Project context that is not derivable from the code (deadlines, incidents, decisions and their rationale)",
		"- Pointers to external systems (dashboards, Linear projects, Slack channels)",
		"- Anything the user explicitly asks you to remember",
		"",
	}
	lines = append(lines, whatNotToSaveSection...)
	lines = append(lines, "")

	if !skipIndex {
		lines = append(lines,
			fmt.Sprintf("## %s", AutoMemEntrypointName),
			fmt.Sprintf("`%s` is the distilled index (maintained nightly from your logs) and is loaded into your context automatically. Read it for orientation, but do not edit it directly — record new information in today's log instead.",
				AutoMemEntrypointName),
			"",
		)
	}

	lines = append(lines, BuildSearchingPastContextSection(memoryDir)...)

	return strings.Join(lines, "\n")
}

// BuildSearchingPastContextSection builds the "Searching past context" section.
func BuildSearchingPastContextSection(autoMemDir string) []string {
	return []string{
		"## Searching past context",
		"",
		"When looking for past context:",
		"1. Search topic files in your memory directory:",
		"```",
		fmt.Sprintf("Grep with pattern=\"<search term>\" path=\"%s\" glob=\"*.md\"", autoMemDir),
		"```",
		"2. Session transcript logs (last resort — large files, slow):",
		"```",
		"Grep with pattern=\"<search term>\" path=\"<project_dir>/\" glob=\"*.jsonl\"",
		"```",
		"Use narrow search terms (error messages, file paths, function names) rather than broad keywords.",
		"",
	}
}

// LoadMemoryPrompt loads the unified memory prompt for the system prompt.
// Returns empty string when auto memory is disabled.
func LoadMemoryPrompt(projectRoot string, kairosActive bool, extraGuidelines []string) string {
	if !IsAutoMemoryEnabled() {
		return ""
	}

	autoDir := GetAutoMemPath(projectRoot)
	_ = EnsureMemoryDirExists(strings.TrimRight(autoDir, string(filepath.Separator)))

	// KAIROS daily-log mode
	if kairosActive {
		return BuildAssistantDailyLogPrompt(autoDir, false)
	}

	return strings.Join(BuildMemoryLines(AutoMemDisplayName, autoDir, extraGuidelines, false), "\n")
}

// ── Prompt content constants ───────────────────────────────────────────────

var memoryFrontmatterExample = []string{
	"```yaml",
	"---",
	"name: descriptive-kebab-case-name",
	"description: One-sentence summary of what this memory contains",
	"type: user | feedback | project | reference",
	"---",
	"```",
}

var typeSectionIndividual = []string{
	"## Memory types",
	"",
	"Each memory file must have a `type` field in its frontmatter. Use one of:",
	"",
	"### `user` — Who the user is",
	"Their role, expertise, preferences, communication style.",
	"Example: \"Senior backend engineer who prefers Go and dislikes ORMs.\"",
	"",
	"### `feedback` — How to collaborate",
	"Direct feedback on your behavior that should change future interactions.",
	"Example: \"Don't add comments unless asked — the user finds them noisy.\"",
	"",
	"### `project` — What the project is",
	"Architecture, key decisions, conventions, and context that is NOT derivable from the code itself.",
	"Example: \"The API gateway rate-limits at 100 req/min per IP; this is an ops decision, not in code.\"",
	"",
	"### `reference` — Useful lookups",
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

var whenToAccessSection = []string{
	"## When to access memories",
	"- At the start of a new conversation to orient yourself",
	"- When the user references something you should already know",
	"- Before making assumptions about preferences or project conventions",
	"- When you need context that might have been discussed in a previous session",
}

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

// ── Helpers ────────────────────────────────────────────────────────────────

func formatFileSize(bytes int) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	kb := float64(bytes) / 1024
	if kb < 1024 {
		return fmt.Sprintf("%.1f KB", kb)
	}
	mb := kb / 1024
	return fmt.Sprintf("%.1f MB", mb)
}
