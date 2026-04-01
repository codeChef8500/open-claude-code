package custom

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/wall-ai/agent-engine/internal/command"
)

// LoadFromSkillDir scans <workDir>/.claude/commands/*.md and returns a slice
// of PromptCommands, one per Markdown file.  Files whose names start with "_"
// are skipped.
func LoadFromSkillDir(workDir string) []command.Command {
	dir := filepath.Join(workDir, ".claude", "commands")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var cmds []command.Command
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, "_") {
			continue
		}

		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		cmdName := strings.ToLower(strings.TrimSuffix(name, ".md"))
		cmds = append(cmds, &skillPromptCommand{
			name:    cmdName,
			content: string(data),
		})
	}
	return cmds
}

// skillPromptCommand is a PromptCommand generated from a Markdown skill file.
type skillPromptCommand struct {
	name    string
	content string
}

func (c *skillPromptCommand) Name() string        { return c.name }
func (c *skillPromptCommand) Description() string { return "Skill: " + c.name }
func (c *skillPromptCommand) Type() command.CommandType {
	return command.CommandTypePrompt
}
func (c *skillPromptCommand) IsEnabled(_ *command.ExecContext) bool { return true }
func (c *skillPromptCommand) PromptContent(args []string, _ *command.ExecContext) (string, error) {
	text := c.content
	if len(args) > 0 {
		text += "\n\nArguments: " + strings.Join(args, " ")
	}
	return text, nil
}

// RegisterSkillDir loads all skill commands from workDir and registers them
// into the given registry.  Already-registered names are silently skipped.
func RegisterSkillDir(r *command.Registry, workDir string) {
	cmds := LoadFromSkillDir(workDir)
	for _, cmd := range cmds {
		// Use a recover to skip duplicate-name panics from Register.
		func() {
			defer func() { recover() }()
			r.Register(cmd)
		}()
	}
}

// NoopExecute satisfies the LocalCommand interface for skill commands that
// should only inject prompt content.  Not used here but exported for callers
// that need a noop executor.
func NoopExecute(_ context.Context, _ []string, _ *command.ExecContext) (string, error) {
	return "", nil
}
