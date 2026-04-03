package permission

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// PathValidator provides comprehensive path validation for the permission
// system, including symlink resolution, sandbox checks, and normalization.
// Aligned with claude-code-main's pathValidation.ts.
type PathValidator struct {
	// workDir is the primary working directory.
	workDir string
	// additionalDirs are extra permitted directories.
	additionalDirs []string
	// resolveSymlinks controls whether symlinks are followed.
	resolveSymlinks bool
}

// NewPathValidator creates a new path validator.
func NewPathValidator(workDir string, additionalDirs []string) *PathValidator {
	return &PathValidator{
		workDir:         workDir,
		additionalDirs:  additionalDirs,
		resolveSymlinks: true,
	}
}

// SetResolveSymlinks controls whether symlinks are resolved during validation.
func (pv *PathValidator) SetResolveSymlinks(v bool) { pv.resolveSymlinks = v }

// ValidatePath checks whether a path is safe and within permitted boundaries.
// Returns the normalized absolute path and any validation error.
func (pv *PathValidator) ValidatePath(path string, write bool) (string, error) {
	// 1. Normalize the path.
	normalized, err := pv.NormalizePath(path)
	if err != nil {
		return "", fmt.Errorf("path validation: normalize: %w", err)
	}

	// 2. Check for basic safety violations.
	if err := pv.checkSafety(normalized); err != nil {
		return "", err
	}

	// 3. For writes, check that path is within permitted directories.
	if write {
		if err := pv.checkWritePermitted(normalized); err != nil {
			return "", err
		}
	}

	// 4. Check for sensitive system paths.
	if write && IsSystemPath(normalized) {
		return "", fmt.Errorf("path validation: %q is a protected system path", normalized)
	}

	return normalized, nil
}

// NormalizePath resolves a path to an absolute, clean form.
// If resolveSymlinks is enabled, symbolic links are resolved.
func (pv *PathValidator) NormalizePath(path string) (string, error) {
	// Make absolute.
	if !filepath.IsAbs(path) {
		if pv.workDir != "" {
			path = filepath.Join(pv.workDir, path)
		} else {
			abs, err := filepath.Abs(path)
			if err != nil {
				return "", err
			}
			path = abs
		}
	}

	// Clean the path (remove . and .. where possible).
	path = filepath.Clean(path)

	// Optionally resolve symlinks.
	if pv.resolveSymlinks {
		resolved, err := resolveSymlinksPartial(path)
		if err != nil {
			// If resolution fails (e.g. file doesn't exist yet), use the cleaned path.
			return path, nil
		}
		return resolved, nil
	}

	return path, nil
}

// IsPermittedDir reports whether a path is within one of the permitted directories.
func (pv *PathValidator) IsPermittedDir(path string) bool {
	normalized, err := pv.NormalizePath(path)
	if err != nil {
		return false
	}
	return pv.isWithinPermitted(normalized)
}

// PermittedDirs returns all permitted directories (work dir + additional).
func (pv *PathValidator) PermittedDirs() []string {
	dirs := make([]string, 0, 1+len(pv.additionalDirs))
	if pv.workDir != "" {
		dirs = append(dirs, pv.workDir)
	}
	dirs = append(dirs, pv.additionalDirs...)
	return dirs
}

// ── Internal checks ─────────────────────────────────────────────────────────

func (pv *PathValidator) checkSafety(path string) error {
	// Block path traversal sequences (should be cleaned by now, belt+suspenders).
	if strings.Contains(path, "..") {
		return fmt.Errorf("path validation: %q contains path traversal", path)
	}

	// Block UNC paths.
	if runtime.GOOS == "windows" {
		if strings.HasPrefix(path, `\\`) {
			return fmt.Errorf("path validation: UNC path %q not permitted", path)
		}
	}

	// Block null bytes.
	if strings.ContainsRune(path, 0) {
		return fmt.Errorf("path validation: %q contains null byte", path)
	}

	return nil
}

func (pv *PathValidator) checkWritePermitted(path string) error {
	if pv.isWithinPermitted(path) {
		return nil
	}

	// Check scratchpad directory.
	home, _ := os.UserHomeDir()
	if home != "" {
		scratchpad := filepath.Join(home, ".claude", "scratch")
		if isUnderPath(path, scratchpad) {
			return nil
		}
		// Also allow writes to ~/.claude/ itself.
		claudeDir := filepath.Join(home, ".claude")
		if isUnderPath(path, claudeDir) {
			return nil
		}
	}

	return fmt.Errorf("path validation: write to %q not permitted (outside allowed directories)", path)
}

func (pv *PathValidator) isWithinPermitted(path string) bool {
	if pv.workDir != "" && isUnderPath(path, pv.workDir) {
		return true
	}
	for _, dir := range pv.additionalDirs {
		if isUnderPath(path, dir) {
			return true
		}
	}
	return false
}

// ── System path detection ───────────────────────────────────────────────────

// systemPathPrefixes are directories that should never be written to.
var systemPathPrefixes = []string{
	"/etc", "/bin", "/sbin", "/usr/bin", "/usr/sbin",
	"/boot", "/sys", "/proc", "/dev",
	"/lib", "/lib64", "/usr/lib", "/usr/lib64",
	"/var/run", "/run",
}

// windowsSystemPrefixes are Windows system paths.
var windowsSystemPrefixes = []string{
	`C:\Windows`, `C:\System32`, `C:\Program Files`,
	`C:\ProgramData`,
}

// IsSystemPath returns true if the path is within a protected system directory.
func IsSystemPath(path string) bool {
	path = filepath.Clean(path)

	for _, prefix := range systemPathPrefixes {
		if isUnderPath(path, prefix) {
			return true
		}
	}

	if runtime.GOOS == "windows" {
		pathUpper := strings.ToUpper(path)
		for _, prefix := range windowsSystemPrefixes {
			if strings.HasPrefix(pathUpper, strings.ToUpper(prefix)) {
				return true
			}
		}
	}

	return false
}

// ── Symlink resolution ──────────────────────────────────────────────────────

// resolveSymlinksPartial resolves symlinks for the existing portion of a path.
// If the full path doesn't exist, it resolves the longest existing prefix and
// appends the remainder.
func resolveSymlinksPartial(path string) (string, error) {
	// Try resolving the full path first.
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}

	// Path doesn't fully exist — resolve parent and append the rest.
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	if dir == path {
		// Reached root — cannot resolve further.
		return path, nil
	}

	resolvedDir, err := resolveSymlinksPartial(dir)
	if err != nil {
		return path, err
	}

	return filepath.Join(resolvedDir, base), nil
}

// IsSymlink reports whether the given path is a symbolic link.
func IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// ResolveSymlinkTarget resolves a symlink to its final target.
func ResolveSymlinkTarget(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

// ── Helper ──────────────────────────────────────────────────────────────────

// isUnderPath reports whether child is equal to or inside parent.
func isUnderPath(child, parent string) bool {
	child = filepath.Clean(child)
	parent = filepath.Clean(parent)
	if child == parent {
		return true
	}
	parentSlash := parent + string(filepath.Separator)
	return strings.HasPrefix(child, parentSlash)
}
