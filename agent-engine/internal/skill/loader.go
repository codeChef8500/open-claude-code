package skill

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/frontmatter"
	"github.com/yuin/goldmark"
)

// SkillMeta is the YAML frontmatter parsed from a skill Markdown file.
type SkillMeta struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Version     string   `yaml:"version"`
	Tags        []string `yaml:"tags"`
	// Allowed tools this skill may invoke.
	AllowedTools []string `yaml:"allowed_tools"`
}

// Skill is a loaded, ready-to-use skill.
type Skill struct {
	Meta    SkillMeta
	Prompt  string // rendered HTML (for LLM injection)
	RawMD   string
	FilePath string
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

// LoadSkillDir scans a directory for *.md files and loads each as a Skill.
func LoadSkillDir(dir string) ([]*Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var skills []*Skill
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
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
