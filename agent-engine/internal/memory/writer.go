package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ClaudeMdWriter provides operations for creating and updating CLAUDE.md files.
type ClaudeMdWriter struct {
	ProjectDir string
}

// NewClaudeMdWriter creates a writer for the given project directory.
func NewClaudeMdWriter(projectDir string) *ClaudeMdWriter {
	return &ClaudeMdWriter{ProjectDir: projectDir}
}

// WriteProjectClaudeMd writes or overwrites the project-level CLAUDE.md.
func (w *ClaudeMdWriter) WriteProjectClaudeMd(content string) error {
	path := filepath.Join(w.ProjectDir, "CLAUDE.md")
	return writeFileSafe(path, content)
}

// WriteLocalClaudeMd writes or overwrites the local .claude/CLAUDE.md.
func (w *ClaudeMdWriter) WriteLocalClaudeMd(content string) error {
	dir := filepath.Join(w.ProjectDir, ".claude")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}
	path := filepath.Join(dir, "CLAUDE.md")
	return writeFileSafe(path, content)
}

// WriteGlobalClaudeMd writes or overwrites the global ~/.claude/CLAUDE.md.
func (w *ClaudeMdWriter) WriteGlobalClaudeMd(content string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create ~/.claude dir: %w", err)
	}
	path := filepath.Join(dir, "CLAUDE.md")
	return writeFileSafe(path, content)
}

// AppendToProjectClaudeMd appends content to the project CLAUDE.md.
func (w *ClaudeMdWriter) AppendToProjectClaudeMd(content string) error {
	path := filepath.Join(w.ProjectDir, "CLAUDE.md")
	return appendToFile(path, content)
}

// AppendToLocalClaudeMd appends content to the local .claude/CLAUDE.md.
func (w *ClaudeMdWriter) AppendToLocalClaudeMd(content string) error {
	dir := filepath.Join(w.ProjectDir, ".claude")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}
	path := filepath.Join(dir, "CLAUDE.md")
	return appendToFile(path, content)
}

// UpdateSection replaces a section in a CLAUDE.md file identified by heading.
// If the section doesn't exist, it is appended.
func (w *ClaudeMdWriter) UpdateSection(level ClaudeMdLevel, heading, newContent string) error {
	path := w.pathForLevel(level)
	if path == "" {
		return fmt.Errorf("unknown level: %s", level)
	}

	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, create with just this section.
			return writeFileSafe(path, heading+"\n\n"+newContent+"\n")
		}
		return err
	}

	text := string(existing)
	updated := replaceSection(text, heading, newContent)
	return writeFileSafe(path, updated)
}

// Exists checks if a CLAUDE.md exists at the given level.
func (w *ClaudeMdWriter) Exists(level ClaudeMdLevel) bool {
	path := w.pathForLevel(level)
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func (w *ClaudeMdWriter) pathForLevel(level ClaudeMdLevel) string {
	switch level {
	case LevelProject:
		return filepath.Join(w.ProjectDir, "CLAUDE.md")
	case LevelLocal:
		return filepath.Join(w.ProjectDir, ".claude", "CLAUDE.md")
	case LevelGlobal:
		home, _ := os.UserHomeDir()
		if home == "" {
			return ""
		}
		return filepath.Join(home, ".claude", "CLAUDE.md")
	}
	return ""
}

// replaceSection finds a markdown heading and replaces everything until the
// next heading of the same or higher level. If not found, appends.
func replaceSection(text, heading, newContent string) string {
	lines := strings.Split(text, "\n")
	headingLevel := countLeadingHashes(heading)
	if headingLevel == 0 {
		// Not a valid heading, just append.
		return text + "\n" + heading + "\n\n" + newContent + "\n"
	}

	startIdx := -1
	endIdx := len(lines)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == strings.TrimSpace(heading) {
			startIdx = i
			continue
		}
		if startIdx >= 0 && i > startIdx {
			level := countLeadingHashes(trimmed)
			if level > 0 && level <= headingLevel {
				endIdx = i
				break
			}
		}
	}

	if startIdx < 0 {
		// Section not found, append.
		return text + "\n\n" + heading + "\n\n" + newContent + "\n"
	}

	// Replace section.
	var result []string
	result = append(result, lines[:startIdx]...)
	result = append(result, heading)
	result = append(result, "")
	result = append(result, newContent)
	result = append(result, "")
	result = append(result, lines[endIdx:]...)

	return strings.Join(result, "\n")
}

func countLeadingHashes(s string) int {
	count := 0
	for _, c := range s {
		if c == '#' {
			count++
		} else {
			break
		}
	}
	return count
}

func writeFileSafe(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func appendToFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("\n" + content)
	return err
}
