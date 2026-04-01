package skill

import (
	"embed"
	"strings"
)

//go:embed bundled_skills/*.md
var bundledFS embed.FS

// BundledSkills loads all skills embedded at compile time from bundled_skills/*.md.
// If no embedded files exist the function returns an empty slice without error.
func BundledSkills() []*Skill {
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
		skills = append(skills, s)
	}
	return skills
}
