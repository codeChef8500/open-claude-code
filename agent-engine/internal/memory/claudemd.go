package memory

import (
	"os"
	"path/filepath"
	"strings"
)

// WalkClaudeMd discovers all CLAUDE.md files relevant to cwd by walking up the
// directory tree from cwd to root.  Files closer to cwd take precedence
// (higher priority).  Results are returned deepest-first.
//
// This matches the claude-code behaviour: every ancestor directory that
// contains a CLAUDE.md contributes to the memory context.
func WalkClaudeMd(cwd string) ([]AncestorFile, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}

	var files []AncestorFile
	dir := abs
	for {
		candidate := filepath.Join(dir, "CLAUDE.md")
		if content, err := readAndExpand(candidate, dir, 0); err == nil && content != "" {
			files = append(files, AncestorFile{
				Path:    candidate,
				Dir:     dir,
				Content: content,
			})
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return files, nil
}

// AncestorFile is a CLAUDE.md file found in an ancestor directory.
type AncestorFile struct {
	Path    string
	Dir     string
	Content string
}

// IsProjectRoot heuristically detects whether dir is the root of a project by
// looking for common markers (go.mod, package.json, .git, pyproject.toml,
// Makefile, Cargo.toml).
func IsProjectRoot(dir string) bool {
	markers := []string{
		"go.mod", "package.json", ".git", "pyproject.toml",
		"Makefile", "Cargo.toml", "CMakeLists.txt", ".hg", ".svn",
	}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			return true
		}
	}
	return false
}

// BuildAncestorContext merges all ancestor CLAUDE.md files into a single
// string, with the deepest (closest to cwd) file first.  A section header is
// prepended for each file when multiple files are present.
func BuildAncestorContext(files []AncestorFile) string {
	if len(files) == 0 {
		return ""
	}
	if len(files) == 1 {
		return files[0].Content
	}
	var sb strings.Builder
	for _, f := range files {
		sb.WriteString("# From: ")
		sb.WriteString(f.Path)
		sb.WriteString("\n\n")
		sb.WriteString(f.Content)
		sb.WriteString("\n\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// FindParentClaudeMdFiles returns the subset of ancestor files that exist in
// directories other than cwd itself (parent-only results).
func FindParentClaudeMdFiles(cwd string) ([]AncestorFile, error) {
	all, err := WalkClaudeMd(cwd)
	if err != nil {
		return nil, err
	}
	absCwd, _ := filepath.Abs(cwd)
	var parents []AncestorFile
	for _, f := range all {
		if filepath.Clean(f.Dir) != filepath.Clean(absCwd) {
			parents = append(parents, f)
		}
	}
	return parents, nil
}
