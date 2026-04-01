package skill

import (
	"os"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
)

// IsConditionallySatisfied returns true if the skill's filePattern (if any)
// matches at least one file in workDir.
// Skills with no filePattern are always active.
func IsConditionallySatisfied(s *Skill, workDir string) bool {
	pattern := s.Meta.FilePattern
	if pattern == "" {
		return true
	}

	// Support both absolute patterns and workDir-relative globs.
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(workDir, pattern)
	}

	matches, err := doublestar.FilepathGlob(pattern)
	if err != nil {
		return false
	}
	return len(matches) > 0
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

// HasFile is a convenience helper that checks whether a path exists under workDir.
func HasFile(workDir, rel string) bool {
	_, err := os.Stat(filepath.Join(workDir, rel))
	return err == nil
}
