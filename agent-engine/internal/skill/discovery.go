package skill

import (
	"os"
	"path/filepath"
	"sync"
)

// SourcePriority defines the precedence when deduplicating skills.
// Higher value = higher priority (overrides lower).
const (
	PriorityBundled = 0
	PriorityManaged = 1
	PriorityUser    = 2
	PriorityProject = 3
	PriorityPlugin  = 4
)

// SkillSource pairs a directory path with its source label and priority.
type SkillSource struct {
	Dir      string
	Source   string
	Priority int
}

var (
	additionalDirsMu sync.RWMutex
	additionalDirs   []SkillSource
)

// AddSkillDirectories registers additional skill directories at runtime
// (e.g., from plugin loading or CLI flags).
func AddSkillDirectories(dirs []SkillSource) {
	additionalDirsMu.Lock()
	defer additionalDirsMu.Unlock()
	additionalDirs = append(additionalDirs, dirs...)
}

// ResetAdditionalDirs clears runtime-added dirs. For testing.
func ResetAdditionalDirs() {
	additionalDirsMu.Lock()
	defer additionalDirsMu.Unlock()
	additionalDirs = nil
}

// DiscoverDirs returns the standard skill directories searched at startup,
// in priority order (lowest to highest):
//  1. ~/.claude/commands/     (user, legacy commands)
//  2. ~/.claude/skills/       (user skills)
//  3. <workDir>/.claude/commands/ (project, legacy commands)
//  4. <workDir>/.claude/skills/   (project skills)
//  5. any additional dirs registered at runtime
func DiscoverDirs(workDir string) []SkillSource {
	var dirs []SkillSource

	home, _ := os.UserHomeDir()
	if home != "" {
		dirs = append(dirs,
			SkillSource{filepath.Join(home, ".claude", "commands"), "user", PriorityUser},
			SkillSource{filepath.Join(home, ".claude", "skills"), "user", PriorityUser},
		)
	}

	if workDir != "" {
		dirs = append(dirs,
			SkillSource{filepath.Join(workDir, ".claude", "commands"), "project", PriorityProject},
			SkillSource{filepath.Join(workDir, ".claude", "skills"), "project", PriorityProject},
		)
	}

	// Append runtime-added dirs.
	additionalDirsMu.RLock()
	dirs = append(dirs, additionalDirs...)
	additionalDirsMu.RUnlock()

	return dirs
}

// DiscoverAll loads skills from all standard discovery directories plus the
// embedded bundled skills. Duplicate names (by skill.Meta.Name) are resolved
// by keeping the higher-priority definition.
func DiscoverAll(workDir string) []*Skill {
	type entry struct {
		skill    *Skill
		priority int
	}
	byName := make(map[string]entry)

	// Bundled skills (lowest priority — can be overridden by user skills).
	for _, s := range BundledSkills() {
		byName[s.Meta.Name] = entry{s, PriorityBundled}
	}

	// User-defined skills from discovery dirs (higher priority).
	for _, src := range DiscoverDirs(workDir) {
		skills, err := LoadSkillDir(src.Dir)
		if err != nil {
			continue
		}
		for _, s := range skills {
			s.Meta.Source = src.Source
			s.Meta.LoadedFrom = src.Dir
			existing, ok := byName[s.Meta.Name]
			if !ok || src.Priority >= existing.priority {
				byName[s.Meta.Name] = entry{s, src.Priority}
			}
		}
	}

	result := make([]*Skill, 0, len(byName))
	for _, e := range byName {
		result = append(result, e.skill)
	}
	return result
}
