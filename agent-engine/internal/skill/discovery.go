package skill

import (
	"os"
	"path/filepath"
)

// DiscoverDirs returns the standard skill directories searched at startup,
// in priority order (lowest to highest):
//  1. ~/.claude/commands/
//  2. <workDir>/.claude/commands/
//  3. <workDir>/CLAUDE.md sibling skills (same directory)
func DiscoverDirs(workDir string) []string {
	var dirs []string

	home, _ := os.UserHomeDir()
	if home != "" {
		dirs = append(dirs, filepath.Join(home, ".claude", "commands"))
	}

	if workDir != "" {
		dirs = append(dirs, filepath.Join(workDir, ".claude", "commands"))
	}

	return dirs
}

// DiscoverAll loads skills from all standard discovery directories plus the
// embedded bundled skills. Duplicate names (by skill.Meta.Name) are resolved
// by keeping the last-seen definition (higher-priority dir wins).
func DiscoverAll(workDir string) []*Skill {
	byName := make(map[string]*Skill)

	// Bundled skills (lowest priority — can be overridden by user skills)
	for _, s := range BundledSkills() {
		byName[s.Meta.Name] = s
	}

	// User-defined skills from discovery dirs (higher priority)
	for _, dir := range DiscoverDirs(workDir) {
		skills, err := LoadSkillDir(dir)
		if err != nil {
			continue
		}
		for _, s := range skills {
			byName[s.Meta.Name] = s
		}
	}

	result := make([]*Skill, 0, len(byName))
	for _, s := range byName {
		result = append(result, s)
	}
	return result
}
