package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// Skill loading infrastructure.
// Aligned with claude-code-main commands/skill-loading pipeline:
//   loadAllProjectSlashCommands → loadSkillsFromDir (project + user)
//   loadBundledSkills → embedded skills from embed/skills/
//   loadMCPSlashCommands → MCP server-provided slash commands
//
// Skills are .md files with YAML frontmatter that define prompt commands.
// They support embedded shell commands (```! ... ``` and !`...`) via the
// shell execution engine.
// ──────────────────────────────────────────────────────────────────────────────

// SkillFrontmatter holds parsed YAML frontmatter from a skill .md file.
type SkillFrontmatter struct {
	Description  string   `json:"description"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
	Shell        string   `json:"shell,omitempty"` // "bash" or "powershell"
	// Feature gating
	RequireFeature string `json:"require_feature,omitempty"`
}

// SkillCommand is a dynamically loaded prompt command from a .md skill file.
type SkillCommand struct {
	BasePromptCommand
	name        string
	description string
	filePath    string
	source      string // "project", "user", "bundled", "mcp"
	frontmatter SkillFrontmatter
}

func (c *SkillCommand) Name() string        { return c.name }
func (c *SkillCommand) Description() string { return c.description }
func (c *SkillCommand) Type() CommandType   { return CommandTypePrompt }
func (c *SkillCommand) IsEnabled(_ *ExecContext) bool {
	if c.frontmatter.RequireFeature != "" {
		// Could check feature flag here; for now always enable.
		return true
	}
	return true
}

// PromptContent reads the skill file and processes shell commands.
func (c *SkillCommand) PromptContent(_ []string, ectx *ExecContext) (string, error) {
	data, err := os.ReadFile(c.filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read skill file %s: %w", c.filePath, err)
	}

	content := string(data)

	// Strip YAML frontmatter.
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content[3:], "---", 2)
		if len(parts) >= 2 {
			content = parts[1]
		}
	}

	content = strings.TrimSpace(content)

	// Execute embedded shell commands if shell service is available.
	if ectx != nil && ectx.Services != nil && ectx.Services.Shell != nil {
		workDir := "."
		if ectx.WorkDir != "" {
			workDir = ectx.WorkDir
		}
		content, _ = ExecuteShellCommandsInPrompt(
			context.Background(), content, workDir, ectx.Services.Shell, 15,
		)
	}

	return content, nil
}

// LoadSkillsFromDir scans a directory for .md skill files and returns commands.
// Aligned with claude-code-main loadSkillsFromDir().
func LoadSkillsFromDir(dir string, source string) []Command {
	if dir == "" {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var cmds []Command
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		cmd, err := loadSkillFile(filePath, source)
		if err != nil {
			continue
		}
		cmds = append(cmds, cmd)
	}

	return cmds
}

// loadSkillFile parses a single .md skill file into a SkillCommand.
func loadSkillFile(path string, source string) (*SkillCommand, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	fm := SkillFrontmatter{}

	// Parse YAML frontmatter.
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content[3:], "---", 2)
		if len(parts) >= 2 {
			frontmatter := parts[0]
			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "description:") {
					fm.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
				} else if strings.HasPrefix(line, "shell:") {
					fm.Shell = strings.TrimSpace(strings.TrimPrefix(line, "shell:"))
				} else if strings.HasPrefix(line, "require_feature:") {
					fm.RequireFeature = strings.TrimSpace(strings.TrimPrefix(line, "require_feature:"))
				} else if strings.HasPrefix(line, "allowed_tools:") {
					// Simple inline array parsing: [tool1, tool2]
					val := strings.TrimSpace(strings.TrimPrefix(line, "allowed_tools:"))
					val = strings.Trim(val, "[]")
					for _, t := range strings.Split(val, ",") {
						t = strings.TrimSpace(t)
						if t != "" {
							fm.AllowedTools = append(fm.AllowedTools, t)
						}
					}
				}
			}
		}
	}

	// Derive command name from filename.
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, ".md")
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")

	desc := fm.Description
	if desc == "" {
		desc = fmt.Sprintf("Skill: %s", name)
	}

	return &SkillCommand{
		name:        name,
		description: desc,
		filePath:    path,
		source:      source,
		frontmatter: fm,
	}, nil
}

// LoadProjectSkills loads skills from the project's .claude/commands/ directory.
// Aligned with claude-code-main loadAllProjectSlashCommands().
func LoadProjectSkills(workDir string) []Command {
	dirs := []struct {
		path   string
		source string
	}{
		{filepath.Join(workDir, ".claude", "commands"), "project"},
		{filepath.Join(workDir, ".claude", "skills"), "project"},
	}

	var cmds []Command
	for _, d := range dirs {
		cmds = append(cmds, LoadSkillsFromDir(d.path, d.source)...)
	}
	return cmds
}

// LoadUserSkills loads skills from the user's global config directory.
func LoadUserSkills() []Command {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	dirs := []struct {
		path   string
		source string
	}{
		{filepath.Join(homeDir, ".claude", "commands"), "user"},
		{filepath.Join(homeDir, ".claude", "skills"), "user"},
	}

	var cmds []Command
	for _, d := range dirs {
		cmds = append(cmds, LoadSkillsFromDir(d.path, d.source)...)
	}
	return cmds
}

// MCPSkillEntry represents a slash command provided by an MCP server.
type MCPSkillEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ServerName  string `json:"server_name"`
}

// MCPSkillCommand wraps an MCP-provided slash command.
type MCPSkillCommand struct {
	BasePromptCommand
	entry MCPSkillEntry
}

func (c *MCPSkillCommand) Name() string                  { return c.entry.Name }
func (c *MCPSkillCommand) Description() string           { return c.entry.Description }
func (c *MCPSkillCommand) Type() CommandType             { return CommandTypePrompt }
func (c *MCPSkillCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *MCPSkillCommand) PromptContent(_ []string, ectx *ExecContext) (string, error) {
	if ectx != nil && ectx.Services != nil && ectx.Services.MCP != nil {
		content, err := ectx.Services.MCP.GetSlashCommandContent(c.entry.ServerName, c.entry.Name)
		if err != nil {
			return fmt.Sprintf("Error loading MCP skill '%s' from server '%s': %v",
				c.entry.Name, c.entry.ServerName, err), nil
		}
		return content, nil
	}
	return fmt.Sprintf("MCP skill '%s' from server '%s' (MCP service unavailable)",
		c.entry.Name, c.entry.ServerName), nil
}

// LoadMCPSkills discovers slash commands from connected MCP servers.
// Aligned with claude-code-main loadMCPSlashCommands().
func LoadMCPSkills(ectx *ExecContext) []Command {
	if ectx == nil || ectx.Services == nil || ectx.Services.MCP == nil {
		return nil
	}

	servers := ectx.Services.MCP.ListServers()
	var cmds []Command

	for _, srv := range servers {
		if srv.Error != "" {
			continue
		}

		slashCmds := ectx.Services.MCP.GetSlashCommands(srv.Name)
		for _, sc := range slashCmds {
			cmds = append(cmds, &MCPSkillCommand{
				entry: MCPSkillEntry{
					Name:        sc.Name,
					Description: sc.Description,
					ServerName:  srv.Name,
				},
			})
		}
	}

	return cmds
}

// RegisterDynamicSkills loads and registers project, user, and MCP skills
// into the registry. This is called during session initialization.
// Aligned with claude-code-main's command loading sequence:
//   1. Built-in commands (already registered)
//   2. Bundled skills (embedded)
//   3. Project skills (.claude/commands/)
//   4. User skills (~/.claude/commands/)
//   5. Plugin commands
//   6. MCP slash commands
func RegisterDynamicSkills(registry *Registry, workDir string, ectx *ExecContext) int {
	registered := 0

	// Project skills.
	for _, cmd := range LoadProjectSkills(workDir) {
		if err := registry.RegisterSafe(cmd); err == nil {
			registered++
		}
	}

	// User skills.
	for _, cmd := range LoadUserSkills() {
		if err := registry.RegisterSafe(cmd); err == nil {
			registered++
		}
	}

	// MCP skills.
	for _, cmd := range LoadMCPSkills(ectx) {
		if err := registry.RegisterSafe(cmd); err == nil {
			registered++
		}
	}

	// Plugin skills (via SkillService).
	if ectx != nil && ectx.Services != nil && ectx.Services.Skill != nil {
		for _, cmd := range ectx.Services.Skill.GetBundledSkills() {
			if err := registry.RegisterSafe(cmd); err == nil {
				registered++
			}
		}
		for _, cmd := range ectx.Services.Skill.GetDynamicSkills() {
			if err := registry.RegisterSafe(cmd); err == nil {
				registered++
			}
		}
	}

	return registered
}
