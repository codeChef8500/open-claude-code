package skill

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// IsConditionallySatisfied returns true if the skill's activation conditions
// are met. A skill is conditional if it has FilePattern or Paths set.
// Skills with no conditions are always active.
func IsConditionallySatisfied(s *Skill, workDir string) bool {
	hasFilePattern := s.Meta.FilePattern != ""
	hasPaths := len(s.Meta.Paths) > 0

	if !hasFilePattern && !hasPaths {
		return true
	}

	// Check legacy FilePattern.
	if hasFilePattern {
		if matchesGlobInDir(s.Meta.FilePattern, workDir) {
			return true
		}
	}

	// Check Paths array.
	for _, p := range s.Meta.Paths {
		if matchesGlobInDir(p, workDir) {
			return true
		}
	}

	return false
}

// IsConditionallySatisfiedForPaths checks whether any of the given file paths
// satisfy a skill's activation conditions. Used for dynamic activation when
// the agent touches specific files.
func IsConditionallySatisfiedForPaths(s *Skill, filePaths []string, workDir string) bool {
	patterns := collectPatterns(s)
	if len(patterns) == 0 {
		return false // not a conditional skill
	}

	for _, fp := range filePaths {
		rel := fp
		if filepath.IsAbs(fp) && workDir != "" {
			if r, err := filepath.Rel(workDir, fp); err == nil {
				rel = r
			}
		}
		// Normalize to forward slashes for doublestar matching.
		rel = filepath.ToSlash(rel)
		for _, pattern := range patterns {
			pattern = filepath.ToSlash(pattern)
			if matched, _ := doublestar.Match(pattern, rel); matched {
				return true
			}
		}
	}
	return false
}

// FilterConditional returns only the skills that are active for workDir.
func FilterConditional(skills []*Skill, workDir string) []*Skill {
	var active []*Skill
	for _, s := range skills {
		if IsConditionallySatisfied(s, workDir) {
			active = append(active, s)
		}
	}
	return active
}

// ActivateConditionalSkillsForPaths checks all skills and returns the names
// of conditional skills that are newly activated by the given file paths.
// This is called after file operations to dynamically activate skills.
func ActivateConditionalSkillsForPaths(skills []*Skill, filePaths []string, workDir string, alreadyActive map[string]bool) []string {
	var newlyActivated []string
	for _, s := range skills {
		if alreadyActive[s.Meta.Name] {
			continue
		}
		if IsConditionallySatisfiedForPaths(s, filePaths, workDir) {
			newlyActivated = append(newlyActivated, s.Meta.Name)
		}
	}
	return newlyActivated
}

// HasFile is a convenience helper that checks whether a path exists under workDir.
func HasFile(workDir, rel string) bool {
	_, err := os.Stat(filepath.Join(workDir, rel))
	return err == nil
}

// collectPatterns returns all activation patterns from a skill.
func collectPatterns(s *Skill) []string {
	var patterns []string
	if s.Meta.FilePattern != "" {
		patterns = append(patterns, s.Meta.FilePattern)
	}
	patterns = append(patterns, s.Meta.Paths...)
	return patterns
}

// matchesGlobInDir checks if a glob pattern matches any file in workDir.
func matchesGlobInDir(pattern, workDir string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}

	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(workDir, pattern)
	}

	matches, err := doublestar.FilepathGlob(pattern)
	if err != nil {
		return false
	}
	return len(matches) > 0
}
