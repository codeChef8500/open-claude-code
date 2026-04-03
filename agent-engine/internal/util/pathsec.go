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

// IsUNCPath reports whether the path is a Windows UNC path (\\server\share or
// //server/share). UNC paths can leak NTLM credentials and must be blocked.
func IsUNCPath(path string) bool {
	return strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//")
}

// blockedDevicePaths contains Linux/macOS device paths that would hang the
// process (infinite output or blocking input) or are nonsensical to read.
var blockedDevicePaths = map[string]bool{
	"/dev/zero":    true,
	"/dev/random":  true,
	"/dev/urandom": true,
	"/dev/full":    true,
	"/dev/stdin":   true,
	"/dev/tty":     true,
	"/dev/console": true,
	"/dev/stdout":  true,
	"/dev/stderr":  true,
	"/dev/fd/0":    true,
	"/dev/fd/1":    true,
	"/dev/fd/2":    true,
}

// IsBlockedDevicePath reports whether filePath is a device file that should
// never be read (infinite output, blocking input, or nonsensical).
func IsBlockedDevicePath(filePath string) bool {
	if blockedDevicePaths[filePath] {
		return true
	}
	// /proc/self/fd/0-2 and /proc/<pid>/fd/0-2 are Linux aliases for stdio.
	if strings.HasPrefix(filePath, "/proc/") &&
		(strings.HasSuffix(filePath, "/fd/0") ||
			strings.HasSuffix(filePath, "/fd/1") ||
			strings.HasSuffix(filePath, "/fd/2")) {
		return true
	}
	return false
}

const (
	// MaxEditFileSize is the maximum file size (in bytes) that the edit tool
	// will process. Matches the V8/Bun ~2^30 char string limit guard.
	MaxEditFileSize = 1 << 30 // 1 GiB

	// MaxReadFileSize is a sensible default for the read tool.
	MaxReadFileSize = 100 * 1024 * 1024 // 100 MiB
)
