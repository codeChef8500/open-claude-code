package skill

import (
	"os"
	"path/filepath"
	"strings"
)

// DiscoverSkillDirsForPaths walks up from each file path looking for
// .claude/skills/ or .claude/commands/ directories, returning any newly
// discovered skill directories. Matches claude-code-main's
// discoverSkillDirsForPaths logic.
func DiscoverSkillDirsForPaths(filePaths []string, workDir string) []string {
	seen := make(map[string]struct{})
	var dirs []string

	// Collect already-known dirs to avoid duplicates.
	for _, src := range DiscoverDirs(workDir) {
		seen[filepath.Clean(src.Dir)] = struct{}{}
	}

	for _, fp := range filePaths {
		if !filepath.IsAbs(fp) {
			fp = filepath.Join(workDir, fp)
		}

		// Walk up from file's directory.
		dir := filepath.Dir(fp)
		for {
			for _, sub := range []string{
				filepath.Join(dir, ".claude", "skills"),
				filepath.Join(dir, ".claude", "commands"),
			} {
				clean := filepath.Clean(sub)
				if _, dup := seen[clean]; dup {
					continue
				}
				if info, err := os.Stat(clean); err == nil && info.IsDir() {
					seen[clean] = struct{}{}
					dirs = append(dirs, clean)
				}
			}

			parent := filepath.Dir(dir)
			if parent == dir {
				break // reached root
			}
			// Stop if we've reached outside workDir.
			if workDir != "" && !strings.HasPrefix(parent, workDir) {
				break
			}
			dir = parent
		}
	}
	return dirs
}

// LoadDynamicSkills loads skills from dynamically discovered directories
// for the given file paths. Returns only new skills not already in existing.
func LoadDynamicSkills(filePaths []string, workDir string, existing map[string]struct{}) []*Skill {
	dirs := DiscoverSkillDirsForPaths(filePaths, workDir)
	var newSkills []*Skill
	for _, dir := range dirs {
		skills, err := LoadSkillDir(dir)
		if err != nil {
			continue
		}
		for _, s := range skills {
			if _, dup := existing[s.Meta.Name]; dup {
				continue
			}
			s.Meta.Source = "project"
			s.Meta.LoadedFrom = dir
			existing[s.Meta.Name] = struct{}{}
			newSkills = append(newSkills, s)
		}
	}
	return newSkills
}
