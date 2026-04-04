package skilltool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// Skill represents a loaded skill definition parsed from a Markdown file
// with YAML frontmatter.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"` // markdown body (instructions)
	Source      string `json:"source"` // file path
}

// Input is the JSON input schema for SkillTool.
type Input struct {
	SkillName string `json:"skill_name"`
}

// SkillTool discovers and executes skill definitions from embedded and
// workspace skill directories.
type SkillTool struct {
	tool.BaseTool
	// embeddedFS is the embedded filesystem containing built-in skills.
	embeddedFS fs.FS
	// workspaceSkillDirs are additional directories to scan for skills.
	workspaceSkillDirs []string
	// cached skills (loaded lazily).
	skills map[string]*Skill
}

// New creates a SkillTool. embeddedFS may be nil if no embedded skills exist.
// workspaceDirs are additional directories to scan for .md skill files.
func New(embeddedFS fs.FS, workspaceDirs ...string) *SkillTool {
	return &SkillTool{
		embeddedFS:         embeddedFS,
		workspaceSkillDirs: workspaceDirs,
	}
}

func (t *SkillTool) Name() string                             { return "Skill" }
func (t *SkillTool) UserFacingName() string                   { return "skill" }
func (t *SkillTool) Description() string                      { return "Execute a predefined skill (reusable workflow)." }
func (t *SkillTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (t *SkillTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *SkillTool) MaxResultSizeChars() int                  { return 50_000 }
func (t *SkillTool) IsEnabled(_ *tool.UseContext) bool        { return true }

func (t *SkillTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"skill_name":{"type":"string","description":"The name of the skill to execute. Use 'list' to see all available skills."}
		},
		"required":["skill_name"]
	}`)
}

func (t *SkillTool) Prompt(_ *tool.UseContext) string {
	return `Execute a predefined skill (reusable workflow defined as a Markdown file with YAML frontmatter).

Skills are loaded from:
1. Built-in embedded skills
2. Workspace .windsurf/skills/ directory
3. Additional configured skill directories

Usage:
- Use skill_name: "list" to discover all available skills
- Each skill provides step-by-step instructions for a specific task
- Skills are read-only — they provide guidance, not direct actions
- The skill's body will be returned as instructions for you to follow`
}

func (t *SkillTool) ValidateInput(_ context.Context, input json.RawMessage) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.SkillName == "" {
		return fmt.Errorf("skill_name must not be empty")
	}
	return nil
}

func (t *SkillTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil
}

func (t *SkillTool) Call(_ context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)

		// Ensure skills are loaded.
		t.loadSkills(uctx)

		// Handle "list" command.
		if in.SkillName == "list" {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: t.listSkills()}
			return
		}

		skill, ok := t.skills[in.SkillName]
		if !ok {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    fmt.Sprintf("Skill %q not found. Use skill_name: \"list\" to see available skills.", in.SkillName),
				IsError: true,
			}
			return
		}

		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: fmt.Sprintf("# Skill: %s\n\n%s\n\n---\n_Source: %s_", skill.Name, skill.Body, skill.Source),
		}
	}()
	return ch, nil
}

// loadSkills discovers and parses skill files from all configured sources.
func (t *SkillTool) loadSkills(uctx *tool.UseContext) {
	if t.skills != nil {
		return
	}
	t.skills = make(map[string]*Skill)

	// 1. Load embedded skills.
	if t.embeddedFS != nil {
		_ = fs.WalkDir(t.embeddedFS, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			data, readErr := fs.ReadFile(t.embeddedFS, path)
			if readErr != nil {
				return nil
			}
			if skill := parseSkillFile(string(data), "embedded:"+path); skill != nil {
				t.skills[skill.Name] = skill
			}
			return nil
		})
	}

	// 2. Load from workspace .windsurf/skills/ directory.
	if uctx != nil && uctx.WorkDir != "" {
		skillDir := filepath.Join(uctx.WorkDir, ".windsurf", "skills")
		t.loadSkillsFromDir(skillDir)
	}

	// 3. Load from additional configured directories.
	for _, dir := range t.workspaceSkillDirs {
		t.loadSkillsFromDir(dir)
	}
}

func (t *SkillTool) loadSkillsFromDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		if skill := parseSkillFile(string(data), fullPath); skill != nil {
			t.skills[skill.Name] = skill
		}
	}
}

func (t *SkillTool) listSkills() string {
	if len(t.skills) == 0 {
		return "No skills available."
	}
	var sb strings.Builder
	sb.WriteString("Available skills:\n\n")
	for _, s := range t.skills {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", s.Name, s.Description))
	}
	return sb.String()
}

// parseSkillFile parses a skill markdown file with YAML frontmatter.
// Expected format:
//
//	---
//	name: skill-name
//	description: A short description
//	---
//	# Skill body in markdown
func parseSkillFile(content, source string) *Skill {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return nil
	}

	// Find the closing frontmatter delimiter.
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return nil
	}

	frontmatter := rest[:idx]
	body := strings.TrimSpace(rest[idx+4:])

	skill := &Skill{
		Body:   body,
		Source: source,
	}

	// Parse simple YAML frontmatter (name, description).
	scanner := bufio.NewScanner(strings.NewReader(frontmatter))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "name:") {
			skill.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		} else if strings.HasPrefix(line, "description:") {
			skill.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		}
	}

	if skill.Name == "" {
		// Derive name from filename.
		base := filepath.Base(source)
		skill.Name = strings.TrimSuffix(base, filepath.Ext(base))
	}

	return skill
}
