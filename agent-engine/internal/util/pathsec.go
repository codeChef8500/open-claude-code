package util

import (
	"path/filepath"
	"strings"
)

// SensitivePaths is the list of paths that should never be written to
// by agent tools, regardless of permissions.
var SensitivePaths = []string{
	"~/.ssh",
	"~/.gnupg",
	"~/.aws",
	"~/.config/gcloud",
	"/etc/passwd",
	"/etc/shadow",
	"/etc/sudoers",
}

// IsPathSafe reports whether filePath is safe for the agent to write.
// A path is unsafe if it contains traversal sequences or matches a sensitive path.
func IsPathSafe(filePath string) bool {
	clean := filepath.Clean(filePath)
	// Reject traversal attempts that escaped upward.
	if strings.Contains(clean, "..") {
		return false
	}
	return !matchesSensitivePath(clean)
}

// IsDirTraversal reports whether target is above base in the filesystem tree.
func IsDirTraversal(base, target string) bool {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false
	}
	return strings.HasPrefix(rel, "..")
}

func matchesSensitivePath(path string) bool {
	normPath := filepath.ToSlash(path)
	for _, sp := range SensitivePaths {
		expanded := filepath.ToSlash(ExpandPath(sp))
		if strings.HasPrefix(normPath, expanded) {
			return true
		}
	}
	return false
}

// SanitizePath removes null bytes and normalises separators.
func SanitizePath(path string) string {
	path = strings.ReplaceAll(path, "\x00", "")
	return filepath.Clean(path)
}
