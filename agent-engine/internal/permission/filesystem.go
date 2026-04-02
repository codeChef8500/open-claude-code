package permission

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FilesystemPermissionChecker validates file write operations against a set
// of permitted directories and detects unsafe path patterns.
type FilesystemPermissionChecker struct {
	// PermittedDirs is the whitelist of directories the agent may write to.
	// If empty, all paths are subject only to safety checks.
	PermittedDirs []string
	// WorkDir is the current working directory (treated as implicitly permitted).
	WorkDir string
}

// CheckWritePermission returns an error if writing to path is not permitted.
func (c *FilesystemPermissionChecker) CheckWritePermission(path string) error {
	clean, err := c.resolvePath(path)
	if err != nil {
		return fmt.Errorf("permission: cannot resolve path %q: %w", path, err)
	}

	// Safety: block path traversal and UNC paths.
	if err := checkPathSafety(clean); err != nil {
		return err
	}

	// If no permitted dirs are configured, only safety checks apply.
	if len(c.PermittedDirs) == 0 && c.WorkDir == "" {
		return nil
	}

	// Check against the working directory.
	if c.WorkDir != "" && isUnder(clean, c.WorkDir) {
		return nil
	}

	// Check against each permitted directory.
	for _, dir := range c.PermittedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if isUnder(clean, absDir) {
			return nil
		}
	}

	return fmt.Errorf("permission: writing to %q is not permitted (outside allowed directories)", path)
}

// CheckReadPermission returns an error if reading from path is not permitted.
// Read checks are less strict — they only block known-sensitive paths.
func (c *FilesystemPermissionChecker) CheckReadPermission(path string) error {
	clean, err := c.resolvePath(path)
	if err != nil {
		return nil // be permissive on resolution failure for reads
	}
	return checkPathSafety(clean)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (c *FilesystemPermissionChecker) resolvePath(path string) (string, error) {
	if !filepath.IsAbs(path) && c.WorkDir != "" {
		path = filepath.Join(c.WorkDir, path)
	}
	return filepath.Abs(path)
}

// isUnder reports whether child is within (or equal to) parent.
func isUnder(child, parent string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	if child == parent {
		return true
	}
	if !strings.HasSuffix(parent, string(filepath.Separator)) {
		parent += string(filepath.Separator)
	}
	return strings.HasPrefix(child, parent)
}

// checkPathSafety blocks known-dangerous path patterns.
func checkPathSafety(path string) error {
	// Block directory traversal sequences (should already be cleaned, belt+suspenders).
	if strings.Contains(path, "..") {
		return fmt.Errorf("permission: path traversal detected in %q", path)
	}

	// Block UNC paths on Windows (\\server\share).
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, `//`) {
		return fmt.Errorf("permission: UNC path %q is not permitted", path)
	}

	// Block writes to critical system paths.
	for _, blocked := range blockedPrefixes {
		if strings.HasPrefix(path, blocked) {
			return fmt.Errorf("permission: writing to system path %q is not permitted", path)
		}
	}
	return nil
}

// blockedPrefixes lists path prefixes that must never be written to.
var blockedPrefixes = []string{
	"/etc/",
	"/bin/",
	"/sbin/",
	"/usr/bin/",
	"/usr/sbin/",
	"/boot/",
	"/sys/",
	"/proc/",
	"/dev/",
	"C:\\Windows\\",
	"C:\\System32\\",
}

// ScratchpadDir returns a session-specific scratch directory path under the
// user's home or the work directory.
func ScratchpadDir(workDir, sessionID string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = workDir
	}
	return filepath.Join(home, ".claude", "scratch", sessionID)
}

// EnsureScratchpad creates the scratchpad directory for a session.
func EnsureScratchpad(workDir, sessionID string) (string, error) {
	dir := ScratchpadDir(workDir, sessionID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("ensure scratchpad: %w", err)
	}
	return dir, nil
}
