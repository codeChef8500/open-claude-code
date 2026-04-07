package teammem

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/wall-ai/agent-engine/internal/memory"
)

// ────────────────────────────────────────────────────────────────────────────
// Team memory path validation — aligned with claude-code-main
// src/memdir/teamMemPaths.ts
// ────────────────────────────────────────────────────────────────────────────

const (
	// TeamMemDirName is the subdirectory name under auto-memory for team memories.
	TeamMemDirName = "team"
	// MaxTeamMemKeyLength is the maximum length for a team memory key.
	MaxTeamMemKeyLength = 200
	// envTeamMemEnabled is the environment variable to control team memory.
	envTeamMemEnabled = "CLAUDE_CODE_TEAM_MEMORY_ENABLED"
)

// IsTeamMemoryEnabled reports whether team memory is enabled.
// Requires auto-memory to be enabled first.
func IsTeamMemoryEnabled() bool {
	if !memory.IsAutoMemoryEnabled() {
		return false
	}
	val := os.Getenv(envTeamMemEnabled)
	if val == "" {
		return false // opt-in feature
	}
	val = strings.ToLower(strings.TrimSpace(val))
	return val == "1" || val == "true" || val == "yes"
}

// GetTeamMemPath returns the team memory directory path.
// It is a subdirectory of the auto-memory path: <autoMemPath>/team/
func GetTeamMemPath(projectRoot string) string {
	autoMemDir := memory.GetAutoMemPath(projectRoot)
	return filepath.Join(strings.TrimRight(autoMemDir, string(filepath.Separator)), TeamMemDirName) + string(filepath.Separator)
}

// ValidateTeamMemKey validates a team memory key for safety.
// Returns an error message if the key is invalid, or empty string if valid.
func ValidateTeamMemKey(key string) string {
	if key == "" {
		return "key must not be empty"
	}
	if len(key) > MaxTeamMemKeyLength {
		return "key is too long"
	}
	// Reject null bytes
	if strings.ContainsRune(key, 0) {
		return "key must not contain null bytes"
	}
	// Reject path traversal
	cleaned := filepath.Clean(key)
	if strings.Contains(cleaned, "..") {
		return "key must not contain path traversal"
	}
	// Reject absolute paths
	if filepath.IsAbs(key) {
		return "key must not be an absolute path"
	}
	// Reject path separators
	if strings.ContainsAny(key, "/\\") {
		return "key must not contain path separators"
	}
	// Reject reserved names on Windows
	if runtime.GOOS == "windows" && isWindowsReserved(key) {
		return "key uses a reserved Windows name"
	}
	return ""
}

// SanitizeTeamMemKey normalises a raw key for use as a team memory filename.
func SanitizeTeamMemKey(raw string) string {
	// Remove extension
	ext := filepath.Ext(raw)
	name := strings.TrimSuffix(raw, ext)

	// Replace unsafe characters
	var sb strings.Builder
	for _, r := range name {
		if isTeamKeySafe(r) {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	result := sb.String()

	// Trim underscores
	result = strings.Trim(result, "_")
	if result == "" {
		result = "_unnamed"
	}

	// Re-add extension (force .md)
	if ext == "" {
		ext = ".md"
	}
	return result + ext
}

// IsTeamMemFile reports whether an absolute path is within the team memory directory.
func IsTeamMemFile(absolutePath, projectRoot string) bool {
	normalized := filepath.Clean(absolutePath)
	teamDir := GetTeamMemPath(projectRoot)
	return isPathUnderDir(normalized, teamDir)
}

// IsTeamMemDir reports whether a directory path is the team memory directory
// or a parent thereof.
func IsTeamMemDir(dirPath, projectRoot string) bool {
	normalized := filepath.Clean(dirPath)
	teamDir := filepath.Clean(strings.TrimRight(GetTeamMemPath(projectRoot), string(filepath.Separator)))
	return toComp(normalized) == toComp(teamDir) || isPathUnderDir(normalized, teamDir+string(filepath.Separator))
}

// ValidateSymlink checks that a symlink target is still contained within
// the team memory directory (PSR M22186 — prevent symlink escape).
func ValidateSymlink(path, projectRoot string) (string, bool) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", false
	}
	teamDir := GetTeamMemPath(projectRoot)
	if !isPathUnderDir(resolved, teamDir) {
		return "", false
	}
	return resolved, true
}

// ── Helpers ────────────────────────────────────────────────────────────────

func isTeamKeySafe(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '-' || r == '_' || r == '.'
}

var windowsReservedRe = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[0-9]|lpt[0-9])(\..+)?$`)

func isWindowsReserved(name string) bool {
	return windowsReservedRe.MatchString(name)
}

func isPathUnderDir(candidate, dir string) bool {
	c := toComp(candidate)
	d := toComp(dir)
	if !strings.HasSuffix(d, "/") {
		d += "/"
	}
	return strings.HasPrefix(c+"/", d) || c == strings.TrimRight(d, "/")
}

func toComp(p string) string {
	result := filepath.ToSlash(p)
	if runtime.GOOS == "windows" {
		result = strings.ToLower(result)
	}
	return result
}
