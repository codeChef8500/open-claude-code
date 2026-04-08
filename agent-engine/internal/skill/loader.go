package skill

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/frontmatter"
	"github.com/yuin/goldmark"
)

// ExecutionContext specifies how a skill is executed.
type ExecutionContext string

const (
	// ExecInline executes the skill inline in the current conversation turn.
	ExecInline ExecutionContext = "inline"
	// ExecFork executes the skill in a forked sub-agent context.
	ExecFork ExecutionContext = "fork"
)

// SkillMeta is the YAML frontmatter parsed from a skill Markdown file.
// Fields mirror claude-code-main's parseSkillFrontmatterFields.
type SkillMeta struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Version     string   `yaml:"version"`
	Tags        []string `yaml:"tags"`
	// AllowedTools restricts which tools this skill may invoke.
	AllowedTools []string `yaml:"allowed_tools"`
	// FilePattern is a glob; if set, the skill is only active when at least
	// one file in the working directory matches (conditional activation).
	FilePattern string `yaml:"file_pattern"`

	// --- Extended fields matching claude-code-main frontmatter ---

	// DisplayName is a user-facing display name (defaults to Name).
	DisplayName string `yaml:"display_name"`
	// WhenToUse is appended to description in the SkillTool prompt listing.
	WhenToUse string `yaml:"when_to_use"`
	// ArgumentHint is shown in the skill listing (e.g., "[file] [flags]").
	ArgumentHint string `yaml:"argument_hint"`
	// Arguments defines named argument names for $name substitution.
	Arguments []string `yaml:"arguments"`
	// Model overrides the LLM model when this skill is active.
	Model string `yaml:"model"`
	// Effort overrides the effort level (e.g., "high", "low").
	Effort string `yaml:"effort"`
	// Shell specifies the shell for embedded shell commands ("bash" or "powershell").
	Shell string `yaml:"shell"`
	// Hooks specifies skill-level hooks (pre_tool_use, post_tool_use, etc.).
	Hooks map[string]interface{} `yaml:"hooks"`
	// Context is the execution context: "inline" (default) or "fork".
	Context ExecutionContext `yaml:"context"`
	// Agent specifies the agent type for forked execution (e.g., "agent").
	Agent string `yaml:"agent"`
	// Paths is a list of path globs for conditional activation.
	Paths []string `yaml:"paths"`
	// UserInvocable indicates whether the user can invoke this skill via slash command.
	UserInvocable *bool `yaml:"user_invocable"`
	// DisableModelInvocation prevents the model from invoking this skill via SkillTool.
	DisableModelInvocation bool `yaml:"disable_model_invocation"`
	// DisableNonInteractive disables the skill in non-interactive (headless) mode.
	DisableNonInteractive bool `yaml:"disable_non_interactive"`

	// --- Source tracking (set at load time, not from frontmatter) ---

	// Source indicates where the skill came from: "bundled", "user", "project", "plugin", "mcp".
	Source string `yaml:"-"`
	// LoadedFrom records the specific path or plugin the skill was loaded from.
	LoadedFrom string `yaml:"-"`
}

// Skill is a loaded, ready-to-use skill.
type Skill struct {
	Meta     SkillMeta
	Prompt   string // rendered HTML (for LLM injection)
	RawMD    string
	FilePath string
	// SkillDir is the parent directory when the skill is a SKILL.md directory.
	// Empty for plain .md file skills.
	SkillDir string
}

var md = goldmark.New()

// LoadSkillFile parses a single Markdown skill file.
func LoadSkillFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var meta SkillMeta
	rest, err := frontmatter.Parse(strings.NewReader(string(data)), &meta)
	if err != nil {
		// No frontmatter — treat entire file as content.
		rest = data
	}

	var buf strings.Builder
	if err := md.Convert(rest, &buf); err != nil {
		buf.WriteString(string(rest))
	}

	// Derive name from filename if not set in frontmatter.
	if meta.Name == "" {
		meta.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	return &Skill{
		Meta:     meta,
		Prompt:   buf.String(),
		RawMD:    string(rest),
		FilePath: path,
	}, nil
}

// ParseSkillBytes parses a Skill from raw bytes and an optional filename hint.
func ParseSkillBytes(data []byte, filename string) (*Skill, error) {
	var meta SkillMeta
	rest, err := frontmatter.Parse(strings.NewReader(string(data)), &meta)
	if err != nil {
		rest = data
	}

	var buf strings.Builder
	if err := md.Convert(rest, &buf); err != nil {
		buf.WriteString(string(rest))
	}

	if meta.Name == "" && filename != "" {
		meta.Name = strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	}

	return &Skill{
		Meta:   meta,
		Prompt: buf.String(),
		RawMD:  string(rest),
	}, nil
}

// LoadSkillDir scans a directory for *.md files and SKILL.md directories,
// loading each as a Skill.
func LoadSkillDir(dir string) ([]*Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var skills []*Skill
	for _, e := range entries {
		if e.IsDir() {
			// Check for SKILL.md convention: a subdirectory containing SKILL.md
			skillMDPath := filepath.Join(dir, e.Name(), "SKILL.md")
			if _, serr := os.Stat(skillMDPath); serr == nil {
				s, lerr := LoadSkillFile(skillMDPath)
				if lerr == nil {
					s.SkillDir = filepath.Join(dir, e.Name())
					// Default name to directory name if not in frontmatter.
					if s.Meta.Name == "SKILL" {
						s.Meta.Name = e.Name()
					}
					skills = append(skills, s)
				}
			}
			continue
		}
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		s, err := LoadSkillFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue // skip malformed skills
		}
		skills = append(skills, s)
	}
	return skills, nil
}
