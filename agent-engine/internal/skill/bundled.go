package skill

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed bundled_skills/*.md
var bundledFS embed.FS

// BundledSkillDefinition describes a programmatic bundled skill, mirroring
// claude-code-main's BundledSkillDefinition. Skills may provide a static
// prompt or a dynamic GetPrompt callback.
type BundledSkillDefinition struct {
	// Name is the slash-command name (e.g., "batch", "remember").
	Name string
	// Description is a short user-facing description.
	Description string
	// WhenToUse is appended to Description in the SkillTool prompt listing.
	WhenToUse string
	// AllowedTools restricts which tools this skill may invoke.
	AllowedTools []string
	// ArgumentHint is shown in the skill listing (e.g., "[pattern]").
	ArgumentHint string
	// Model overrides the LLM model.
	Model string
	// Effort overrides the effort level.
	Effort string
	// Context is the execution context (inline/fork).
	Context ExecutionContext
	// Agent specifies the agent type for forked execution.
	Agent string
	// DisableModelInvocation prevents model from invoking via SkillTool.
	DisableModelInvocation bool
	// DisableNonInteractive disables in headless mode.
	DisableNonInteractive bool
	// IsEnabled is called to check if this skill should be available.
	// Nil means always enabled.
	IsEnabled func() bool
	// GetPrompt returns the prompt text dynamically. If nil, StaticPrompt is used.
	GetPrompt func(args string) string
	// StaticPrompt is the fixed prompt text (used when GetPrompt is nil).
	StaticPrompt string
	// Files maps relative paths to file contents that are extracted to disk
	// on first invocation (~/.claude/bundled-skills/<name>/).
	Files map[string][]byte
}

var (
	bundledDefsMu sync.RWMutex
	bundledDefs   []BundledSkillDefinition
)

// RegisterBundledSkill adds a programmatic bundled skill definition.
func RegisterBundledSkill(def BundledSkillDefinition) {
	bundledDefsMu.Lock()
	defer bundledDefsMu.Unlock()
	bundledDefs = append(bundledDefs, def)
}

// GetBundledSkillDefs returns a copy of all registered programmatic bundled skill definitions.
func GetBundledSkillDefs() []BundledSkillDefinition {
	bundledDefsMu.RLock()
	defer bundledDefsMu.RUnlock()
	out := make([]BundledSkillDefinition, len(bundledDefs))
	copy(out, bundledDefs)
	return out
}

// BundledSkills loads all skills: embedded markdown files + programmatic definitions.
// Programmatic definitions take precedence over embedded files with the same name.
func BundledSkills() []*Skill {
	// Load embedded markdown skills.
	embeddedSkills := loadEmbeddedSkills()

	// Convert programmatic definitions to skills.
	defs := GetBundledSkillDefs()
	progSkills := make([]*Skill, 0, len(defs))
	for _, def := range defs {
		if def.IsEnabled != nil && !def.IsEnabled() {
			continue
		}
		s := bundledDefToSkill(def)
		progSkills = append(progSkills, s)
	}

	// Merge: programmatic wins on name collision.
	nameSet := make(map[string]struct{})
	var merged []*Skill
	for _, s := range progSkills {
		nameSet[s.Meta.Name] = struct{}{}
		merged = append(merged, s)
	}
	for _, s := range embeddedSkills {
		if _, dup := nameSet[s.Meta.Name]; !dup {
			merged = append(merged, s)
		}
	}
	return merged
}

// loadEmbeddedSkills reads skills from the bundled_skills embed.FS.
func loadEmbeddedSkills() []*Skill {
	entries, err := bundledFS.ReadDir("bundled_skills")
	if err != nil {
		return nil
	}
	var skills []*Skill
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := bundledFS.ReadFile("bundled_skills/" + e.Name())
		if err != nil {
			continue
		}
		s, err := ParseSkillBytes(data, e.Name())
		if err != nil {
			continue
		}
		s.Meta.Source = "bundled"
		skills = append(skills, s)
	}
	return skills
}

// bundledDefToSkill converts a BundledSkillDefinition to a Skill.
func bundledDefToSkill(def BundledSkillDefinition) *Skill {
	prompt := def.StaticPrompt
	if def.GetPrompt != nil {
		prompt = def.GetPrompt("")
	}

	userInvocable := true
	return &Skill{
		Meta: SkillMeta{
			Name:                   def.Name,
			Description:            def.Description,
			WhenToUse:              def.WhenToUse,
			AllowedTools:           def.AllowedTools,
			ArgumentHint:           def.ArgumentHint,
			Model:                  def.Model,
			Effort:                 def.Effort,
			Context:                def.Context,
			Agent:                  def.Agent,
			DisableModelInvocation: def.DisableModelInvocation,
			DisableNonInteractive:  def.DisableNonInteractive,
			UserInvocable:          &userInvocable,
			Source:                 "bundled",
		},
		Prompt: prompt,
		RawMD:  prompt,
	}
}

// ExtractBundledSkillFiles writes a bundled skill's reference files to disk.
// Extracts to ~/.claude/bundled-skills/<name>/ and returns the directory path.
func ExtractBundledSkillFiles(name string, files map[string][]byte) (string, error) {
	if len(files) == 0 {
		return "", nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(homeDir, ".claude", "bundled-skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	for relPath, content := range files {
		absPath := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(absPath, content, 0o644); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// ResetBundledSkills clears all registered programmatic bundled skill definitions.
// Intended for testing.
func ResetBundledSkills() {
	bundledDefsMu.Lock()
	defer bundledDefsMu.Unlock()
	bundledDefs = nil
}
