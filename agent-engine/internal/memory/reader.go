package memory

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var includeRe = regexp.MustCompile(`(?m)^@include\s+(.+)$`)

// ReadClaudeMd loads all CLAUDE.md files in order of precedence:
//  1. Global  : ~/.claude/CLAUDE.md
//  2. Project : <cwd>/CLAUDE.md
//  3. Local   : <cwd>/.claude/CLAUDE.md
//
// @include directives inside each file are expanded recursively.
func ReadClaudeMd(cwd string) (*MemoryInjection, error) {
	home, _ := os.UserHomeDir()

	paths := []struct {
		level    ClaudeMdLevel
		filePath string
	}{
		{LevelGlobal, filepath.Join(home, ".claude", "CLAUDE.md")},
		{LevelProject, filepath.Join(cwd, "CLAUDE.md")},
		{LevelLocal, filepath.Join(cwd, ".claude", "CLAUDE.md")},
	}

	injection := &MemoryInjection{}

	for _, p := range paths {
		content, err := readAndExpand(p.filePath, cwd, 0)
		if err != nil {
			continue // file not found or unreadable — skip
		}
		switch p.level {
		case LevelGlobal:
			injection.GlobalContent = content
		case LevelProject:
			injection.ProjectContent = content
		case LevelLocal:
			injection.LocalContent = content
		}
	}

	return injection, nil
}

// MergedContent returns all non-empty CLAUDE.md contents joined with newlines.
func (m *MemoryInjection) MergedContent() string {
	var parts []string
	if m.GlobalContent != "" {
		parts = append(parts, m.GlobalContent)
	}
	if m.ProjectContent != "" {
		parts = append(parts, m.ProjectContent)
	}
	if m.LocalContent != "" {
		parts = append(parts, m.LocalContent)
	}
	return strings.Join(parts, "\n\n")
}

// HasContent reports whether any CLAUDE.md content was loaded.
func (m *MemoryInjection) HasContent() bool {
	return m.GlobalContent != "" || m.ProjectContent != "" || m.LocalContent != ""
}

// readAndExpand reads a file and resolves @include directives (max depth 5).
func readAndExpand(filePath, baseDir string, depth int) (string, error) {
	if depth > 5 {
		return "", nil
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	text := string(data)
	fileDir := filepath.Dir(filePath)

	// Expand @include directives
	text = includeRe.ReplaceAllStringFunc(text, func(match string) string {
		sub := includeRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		includePath := strings.TrimSpace(sub[1])
		if !filepath.IsAbs(includePath) {
			includePath = filepath.Join(fileDir, includePath)
		}
		included, err := readAndExpand(includePath, baseDir, depth+1)
		if err != nil {
			return match + " [include not found]"
		}
		return included
	})

	return text, nil
}
