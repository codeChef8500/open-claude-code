package custom

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/wall-ai/agent-engine/internal/command"
)

// SkillFrontmatter holds optional YAML frontmatter from a skill .md file.
// Aligned with claude-code-main's skill command metadata parsing.
//
// Frontmatter format (at top of .md file):
//
//	---
//	description: Short description of the skill
//	argument_hint: <required-arg> [optional-arg]
//	aliases: alias1, alias2
//	hidden: false
//	allowed_tools: tool1, tool2
//	---
type SkillFrontmatter struct {
	Description  string
	ArgumentHint string
	Aliases      []string
	Hidden       bool
	AllowedTools []string
}

// parseFrontmatter extracts YAML-like frontmatter from markdown content.
// Returns the frontmatter and the remaining body content.
func parseFrontmatter(content string) (SkillFrontmatter, string) {
	var fm SkillFrontmatter

	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return fm, content
	}

	// Find closing ---
	rest := trimmed[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return fm, content
	}

	fmBlock := rest[:idx]
	body := strings.TrimSpace(rest[idx+4:])

	// Parse simple key: value pairs
	for _, line := range strings.Split(fmBlock, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(parts[0]))
		val := strings.TrimSpace(parts[1])
		switch key {
		case "description":
			fm.Description = val
		case "argument_hint", "argumenthint":
			fm.ArgumentHint = val
		case "aliases":
			for _, a := range strings.Split(val, ",") {
				a = strings.TrimSpace(a)
				if a != "" {
					fm.Aliases = append(fm.Aliases, a)
				}
			}
		case "hidden":
			fm.Hidden = strings.ToLower(val) == "true" || val == "1"
		case "allowed_tools", "allowedtools":
			for _, t := range strings.Split(val, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					fm.AllowedTools = append(fm.AllowedTools, t)
				}
			}
		}
	}

	return fm, body
}

// LoadFromSkillDir scans a directory for .md skill files and returns commands.
// Files whose names start with "_" are skipped. Supports subdirectories
// (nested commands become "subdir:name" format).
func LoadFromSkillDir(dir string) []command.Command {
	return loadSkillsRecursive(dir, "")
}

func loadSkillsRecursive(dir, prefix string) []command.Command {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var cmds []command.Command
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}

		if e.IsDir() {
			subPrefix := name
			if prefix != "" {
				subPrefix = prefix + ":" + name
			}
			cmds = append(cmds, loadSkillsRecursive(filepath.Join(dir, name), subPrefix)...)
			continue
		}

		if !strings.HasSuffix(name, ".md") {
			continue
		}

		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		cmdName := strings.ToLower(strings.TrimSuffix(name, ".md"))
		if prefix != "" {
			cmdName = prefix + ":" + cmdName
		}

		fm, body := parseFrontmatter(string(data))

		cmd := &skillPromptCommand{
			cmdName:     cmdName,
			description: fm.Description,
			argHint:     fm.ArgumentHint,
			aliases:     fm.Aliases,
			hidden:      fm.Hidden,
			body:        body,
			loadedFrom:  path,
		}
		cmds = append(cmds, cmd)
	}
	return cmds
}

// LoadFromAllSkillDirs loads skills from both project and user-home directories.
// Aligned with claude-code-main loadAllCommands() which merges:
//   - <workDir>/.claude/commands/
//   - ~/.claude/commands/
func LoadFromAllSkillDirs(workDir string) []command.Command {
	var all []command.Command

	// Project-level skills
	projectDir := filepath.Join(workDir, ".claude", "commands")
	all = append(all, LoadFromSkillDir(projectDir)...)

	// User-level skills
	home, err := os.UserHomeDir()
	if err == nil {
		userDir := filepath.Join(home, ".claude", "commands")
		all = append(all, LoadFromSkillDir(userDir)...)
	}

	return all
}

// NewSkillDirLoader returns a DynamicLoader that can be registered with
// Registry.AddLoader() for automatic discovery.
func NewSkillDirLoader(workDir string) command.DynamicLoader {
	return func() []command.Command {
		return LoadFromAllSkillDirs(workDir)
	}
}

// skillPromptCommand is a PromptCommand generated from a Markdown skill file.
type skillPromptCommand struct {
	command.BasePromptCommand
	cmdName     string
	description string
	argHint     string
	aliases     []string
	hidden      bool
	body        string
	loadedFrom  string
}

func (c *skillPromptCommand) Name() string { return c.cmdName }
func (c *skillPromptCommand) Description() string {
	if c.description != "" {
		return c.description
	}
	return "Skill: " + c.cmdName
}
func (c *skillPromptCommand) ArgumentHint() string { return c.argHint }
func (c *skillPromptCommand) Aliases() []string    { return c.aliases }
func (c *skillPromptCommand) IsHidden() bool       { return c.hidden }
func (c *skillPromptCommand) Source() command.CommandSource {
	return command.CommandSourceCustom
}
func (c *skillPromptCommand) LoadedFrom() command.CommandLoadedFrom { return command.LoadedFromSkills }
func (c *skillPromptCommand) Type() command.CommandType {
	return command.CommandTypePrompt
}
func (c *skillPromptCommand) IsEnabled(_ *command.ExecContext) bool { return true }
func (c *skillPromptCommand) PromptContent(args []string, _ *command.ExecContext) (string, error) {
	text := c.body
	if len(args) > 0 {
		text += "\n\nArguments: " + strings.Join(args, " ")
	}
	return text, nil
}

// RegisterSkillDir loads all skill commands from workDir and registers them
// into the given registry using RegisterOrReplace semantics.
func RegisterSkillDir(r *command.Registry, workDir string) {
	cmds := LoadFromAllSkillDirs(workDir)
	if len(cmds) > 0 {
		r.RegisterOrReplace(cmds...)
	}
}

// NoopExecute satisfies the LocalCommand interface for skill commands that
// should only inject prompt content.  Not used here but exported for callers
// that need a noop executor.
func NoopExecute(_ context.Context, _ []string, _ *command.ExecContext) (string, error) {
	return "", nil
}
