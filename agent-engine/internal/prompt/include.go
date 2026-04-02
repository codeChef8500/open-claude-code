package prompt

import (
	"os"
	"path/filepath"
	"strings"
)

const maxIncludeDepth = 5

// ResolveIncludes processes `@include <path>` directives in a CLAUDE.md file,
// recursively substituting each directive with the referenced file's content.
//
// Includes are relative to the directory of the file containing the directive.
// Circular includes and depth > maxIncludeDepth are silently skipped.
//
// Format (must appear at the start of a line):
//
//	@include ./relative/path.md
//	@include /absolute/path.md
func ResolveIncludes(content, baseDir string) string {
	return resolveIncludes(content, baseDir, 0, make(map[string]struct{}))
}

func resolveIncludes(content, baseDir string, depth int, seen map[string]struct{}) string {
	if depth > maxIncludeDepth {
		return content
	}

	lines := strings.Split(content, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "@include ") {
			out = append(out, line)
			continue
		}

		includePath := strings.TrimSpace(strings.TrimPrefix(trimmed, "@include "))
		if includePath == "" {
			out = append(out, line)
			continue
		}

		// Resolve to absolute path.
		if !filepath.IsAbs(includePath) {
			includePath = filepath.Join(baseDir, includePath)
		}
		includePath = filepath.Clean(includePath)

		// Guard against circular includes.
		if _, already := seen[includePath]; already {
			out = append(out, "<!-- @include cycle detected: "+includePath+" -->")
			continue
		}

		data, err := os.ReadFile(includePath)
		if err != nil {
			out = append(out, "<!-- @include error: "+err.Error()+" -->")
			continue
		}

		// Recurse into the included file.
		seen[includePath] = struct{}{}
		included := resolveIncludes(string(data), filepath.Dir(includePath), depth+1, seen)
		delete(seen, includePath)

		out = append(out, included)
	}
	return strings.Join(out, "\n")
}

// LoadClaudeMD reads the CLAUDE.md file from workDir (or any parent up to the
// filesystem root), resolves @include directives, and returns the merged
// content.  Returns "" and no error if no CLAUDE.md is found.
func LoadClaudeMD(workDir string) (string, error) {
	path, err := findClaudeMD(workDir)
	if err != nil || path == "" {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	resolved := ResolveIncludes(string(data), filepath.Dir(path))
	return resolved, nil
}

// findClaudeMD walks up from workDir looking for a CLAUDE.md file.
func findClaudeMD(workDir string) (string, error) {
	dir := workDir
	for {
		candidate := filepath.Join(dir, "CLAUDE.md")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}
	return "", nil
}
