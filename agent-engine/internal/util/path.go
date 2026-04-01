package util

import (
	"os"
	"path/filepath"
	"strings"
)

// ExpandPath expands a leading ~ to the user's home directory.
// ~/foo  →  /home/user/foo
// ~      →  /home/user
// /abs   →  /abs (unchanged)
func ExpandPath(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// IsPathUnderTrusted reports whether filePath is located inside at least one
// of the trustedDirs. Both paths are resolved to absolute form before comparison
// to prevent path-traversal attacks.
func IsPathUnderTrusted(filePath string, trustedDirs []string) bool {
	resolved, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}
	// Normalize to use forward slashes for consistent prefix matching.
	resolved = filepath.ToSlash(resolved)

	for _, dir := range trustedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		absDir = filepath.ToSlash(absDir)
		// Ensure the separator is included so /foo doesn't match /foobar.
		if !strings.HasSuffix(absDir, "/") {
			absDir += "/"
		}
		if strings.HasPrefix(resolved+"/", absDir) {
			return true
		}
	}
	return false
}

// CleanPath cleans and converts the path to the OS-native separator.
func CleanPath(path string) string {
	return filepath.Clean(path)
}

// JoinPath joins path elements and cleans the result.
func JoinPath(elem ...string) string {
	return filepath.Join(elem...)
}

// DirOf returns the directory portion of a path.
func DirOf(path string) string {
	return filepath.Dir(path)
}

// BaseOf returns the last element (filename) of a path.
func BaseOf(path string) string {
	return filepath.Base(path)
}

// ExtOf returns the file extension including the dot (e.g. ".go").
func ExtOf(path string) string {
	return filepath.Ext(path)
}

// AbsPath returns the absolute path, resolving relative components.
func AbsPath(path string) (string, error) {
	return filepath.Abs(path)
}
