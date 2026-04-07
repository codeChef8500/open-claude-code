package memory

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"unicode"
)

// ────────────────────────────────────────────────────────────────────────────
// Auto-memory path system — aligned with claude-code-main src/memdir/paths.ts
// ────────────────────────────────────────────────────────────────────────────

const (
	// AutoMemDirName is the directory name for per-project auto-memory.
	AutoMemDirName = "memory"
	// AutoMemEntrypointName is the index file inside the auto-memory dir.
	AutoMemEntrypointName = "MEMORY.md"
	// ProjectsDirName is the intermediate directory under the memory base.
	ProjectsDirName = "projects"
)

// ── Environment variable keys ──────────────────────────────────────────────

const (
	envDisableAutoMemory = "CLAUDE_CODE_DISABLE_AUTO_MEMORY"
	envSimple            = "CLAUDE_CODE_SIMPLE"
	envRemote            = "CLAUDE_CODE_REMOTE"
	envRemoteMemoryDir   = "CLAUDE_CODE_REMOTE_MEMORY_DIR"
	envMemoryPathOverride = "CLAUDE_COWORK_MEMORY_PATH_OVERRIDE"
	envClaudeConfigDir   = "CLAUDE_CONFIG_DIR"
)

// ── Core path functions ────────────────────────────────────────────────────

// GetClaudeConfigHomeDir returns the Claude configuration home directory.
// Priority: CLAUDE_CONFIG_DIR env var → ~/.claude
func GetClaudeConfigHomeDir() string {
	if dir := os.Getenv(envClaudeConfigDir); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".claude")
	}
	return filepath.Join(home, ".claude")
}

// GetMemoryBaseDir returns the base directory for persistent memory storage.
// Resolution order:
//  1. CLAUDE_CODE_REMOTE_MEMORY_DIR env var (explicit override, set in CCR)
//  2. ~/.claude (default config home)
func GetMemoryBaseDir() string {
	if dir := os.Getenv(envRemoteMemoryDir); dir != "" {
		return dir
	}
	return GetClaudeConfigHomeDir()
}

// IsAutoMemoryEnabled reports whether auto-memory features are enabled.
// Enabled by default. Priority chain (first defined wins):
//  1. CLAUDE_CODE_DISABLE_AUTO_MEMORY env var (1/true → OFF, 0/false → ON)
//  2. CLAUDE_CODE_SIMPLE (--bare) → OFF
//  3. CCR without persistent storage → OFF (no CLAUDE_CODE_REMOTE_MEMORY_DIR)
//  4. Default: enabled
func IsAutoMemoryEnabled() bool {
	if isEnvTruthy(os.Getenv(envDisableAutoMemory)) {
		return false
	}
	if isEnvDefinedFalsy(os.Getenv(envDisableAutoMemory)) {
		return true
	}
	if isEnvTruthy(os.Getenv(envSimple)) {
		return false
	}
	if isEnvTruthy(os.Getenv(envRemote)) && os.Getenv(envRemoteMemoryDir) == "" {
		return false
	}
	return true
}

// IsExtractModeActive reports whether the extract-memories background agent
// will run this session.
func IsExtractModeActive() bool {
	// In Go port, controlled by feature config. Default to enabled when
	// auto-memory is enabled.
	return IsAutoMemoryEnabled()
}

// ── Auto-memory path resolution ────────────────────────────────────────────

// autoMemPathCache caches the resolved auto-memory path per project root.
var (
	autoMemPathCache   = make(map[string]string)
	autoMemPathCacheMu sync.RWMutex
)

// ClearAutoMemPathCache clears the cached auto-memory paths. Call after
// settings or env vars change mid-session.
func ClearAutoMemPathCache() {
	autoMemPathCacheMu.Lock()
	defer autoMemPathCacheMu.Unlock()
	autoMemPathCache = make(map[string]string)
}

// GetAutoMemPath returns the auto-memory directory path.
// Resolution order:
//  1. CLAUDE_COWORK_MEMORY_PATH_OVERRIDE env var (full-path override)
//  2. <memoryBase>/projects/<sanitized-project-root>/memory/
func GetAutoMemPath(projectRoot string) string {
	autoMemPathCacheMu.RLock()
	if cached, ok := autoMemPathCache[projectRoot]; ok {
		autoMemPathCacheMu.RUnlock()
		return cached
	}
	autoMemPathCacheMu.RUnlock()

	result := resolveAutoMemPath(projectRoot)

	autoMemPathCacheMu.Lock()
	autoMemPathCache[projectRoot] = result
	autoMemPathCacheMu.Unlock()

	return result
}

func resolveAutoMemPath(projectRoot string) string {
	// Priority 1: env override
	if override := validateMemoryPath(os.Getenv(envMemoryPathOverride), false); override != "" {
		return override
	}
	// Priority 2: computed path
	projectsDir := filepath.Join(GetMemoryBaseDir(), ProjectsDirName)
	result := filepath.Join(projectsDir, SanitizePathForMemory(projectRoot), AutoMemDirName)
	return ensureTrailingSep(result)
}

// GetAutoMemDailyLogPath returns the daily log file path for the given date.
// Shape: <autoMemPath>/logs/YYYY/MM/YYYY-MM-DD.md
func GetAutoMemDailyLogPath(projectRoot string, year int, month int, day int) string {
	mm := padTwo(month)
	dd := padTwo(day)
	yyyy := padFour(year)
	return filepath.Join(GetAutoMemPath(projectRoot), "logs", yyyy, mm, yyyy+"-"+mm+"-"+dd+".md")
}

// GetAutoMemEntrypoint returns the auto-memory entrypoint (MEMORY.md).
func GetAutoMemEntrypoint(projectRoot string) string {
	return filepath.Join(GetAutoMemPath(projectRoot), AutoMemEntrypointName)
}

// HasAutoMemPathOverride reports whether CLAUDE_COWORK_MEMORY_PATH_OVERRIDE
// is set to a valid override.
func HasAutoMemPathOverride() bool {
	return validateMemoryPath(os.Getenv(envMemoryPathOverride), false) != ""
}

// IsAutoMemPath checks if an absolute path is within the auto-memory directory.
func IsAutoMemPath(absolutePath, projectRoot string) bool {
	normalized := filepath.Clean(absolutePath)
	autoMemDir := GetAutoMemPath(projectRoot)
	return isPathUnder(normalized, autoMemDir)
}

// ── FindCanonicalGitRoot ───────────────────────────────────────────────────

// FindCanonicalGitRoot returns the canonical git root for a directory.
// For worktrees, it returns the main working tree root so all worktrees
// share one auto-memory directory. Returns "" if not in a git repo.
func FindCanonicalGitRoot(dir string) string {
	// Walk up looking for .git
	current := filepath.Clean(dir)
	for {
		gitDir := filepath.Join(current, ".git")
		info, err := os.Stat(gitDir)
		if err == nil {
			if info.IsDir() {
				// Regular git repo
				return current
			}
			// gitdir file (worktree) — read the real path
			data, err := os.ReadFile(gitDir)
			if err == nil {
				line := strings.TrimSpace(string(data))
				if strings.HasPrefix(line, "gitdir: ") {
					gitPath := strings.TrimPrefix(line, "gitdir: ")
					if !filepath.IsAbs(gitPath) {
						gitPath = filepath.Join(current, gitPath)
					}
					// .git/worktrees/<name> → go up 2 levels to find main root
					resolved := filepath.Clean(gitPath)
					// Check if this is a worktree reference
					if strings.Contains(resolved, filepath.Join(".git", "worktrees")) {
						mainGitDir := resolved
						for i := 0; i < 2; i++ {
							mainGitDir = filepath.Dir(mainGitDir)
						}
						// mainGitDir should now be the .git directory
						mainRoot := filepath.Dir(mainGitDir)
						if _, err := os.Stat(filepath.Join(mainRoot, ".git")); err == nil {
							return mainRoot
						}
					}
				}
			}
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return ""
}

// ── Path validation & sanitization ─────────────────────────────────────────

// validateMemoryPath normalises and validates a candidate auto-memory directory path.
// SECURITY: Rejects dangerous paths (relative, root, UNC, null byte).
// Returns the normalised path with trailing separator, or "" if rejected.
func validateMemoryPath(raw string, expandTilde bool) string {
	if raw == "" {
		return ""
	}

	candidate := raw

	// Tilde expansion (settings.json paths support ~/)
	if expandTilde && (strings.HasPrefix(candidate, "~/") || strings.HasPrefix(candidate, "~\\")) {
		rest := candidate[2:]
		// Reject trivial remainders that would expand to $HOME or ancestor
		restClean := filepath.Clean(rest)
		if restClean == "." || restClean == ".." || rest == "" {
			return ""
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		candidate = filepath.Join(home, rest)
	}

	// Normalise: strip trailing separator, then clean
	normalized := filepath.Clean(candidate)
	normalized = strings.TrimRight(normalized, string(filepath.Separator))
	if runtime.GOOS == "windows" {
		normalized = strings.TrimRight(normalized, "/")
	}

	// Security checks
	if !filepath.IsAbs(normalized) {
		return ""
	}
	if len(normalized) < 3 {
		return ""
	}
	// Windows drive-root check (C:)
	if runtime.GOOS == "windows" && driveRootRe.MatchString(normalized) {
		return ""
	}
	// UNC path check
	if strings.HasPrefix(normalized, `\\`) || strings.HasPrefix(normalized, "//") {
		return ""
	}
	// Null byte check
	if strings.ContainsRune(normalized, 0) {
		return ""
	}

	return ensureTrailingSep(normalized)
}

var driveRootRe = regexp.MustCompile(`^[A-Za-z]:$`)

// SanitizePathForMemory converts an absolute path into a safe, flat directory name
// for use as a memory sub-directory key.
func SanitizePathForMemory(path string) string {
	if path == "" {
		return "_empty"
	}
	// Clean the path
	cleaned := filepath.Clean(path)
	// On Windows, remove the drive letter prefix
	if runtime.GOOS == "windows" && len(cleaned) >= 2 && cleaned[1] == ':' {
		cleaned = cleaned[0:1] + cleaned[2:]
	}
	// Replace path separators and unsafe characters with underscores
	var sb strings.Builder
	for _, r := range cleaned {
		switch {
		case r == filepath.Separator || r == '/' || r == '\\':
			sb.WriteRune('_')
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '.':
			sb.WriteRune(r)
		default:
			sb.WriteRune('_')
		}
	}
	result := sb.String()
	// Trim leading/trailing underscores
	result = strings.Trim(result, "_")
	if result == "" {
		return "_root"
	}
	return result
}

// ── Helpers ────────────────────────────────────────────────────────────────

func isEnvTruthy(val string) bool {
	val = strings.ToLower(strings.TrimSpace(val))
	return val == "1" || val == "true" || val == "yes"
}

func isEnvDefinedFalsy(val string) bool {
	if val == "" {
		return false
	}
	val = strings.ToLower(strings.TrimSpace(val))
	return val == "0" || val == "false" || val == "no"
}

func ensureTrailingSep(path string) string {
	sep := string(filepath.Separator)
	if !strings.HasSuffix(path, sep) {
		return path + sep
	}
	return path
}

func isPathUnder(candidate, dir string) bool {
	// Normalise both to comparable form
	candidateNorm := toComparable(candidate)
	dirNorm := toComparable(dir)
	// Ensure dir has trailing separator
	if !strings.HasSuffix(dirNorm, "/") {
		dirNorm += "/"
	}
	return strings.HasPrefix(candidateNorm+"/", dirNorm) || candidateNorm == strings.TrimRight(dirNorm, "/")
}

// toComparable converts a path to forward-slash form; on Windows also lowercases.
func toComparable(p string) string {
	result := filepath.ToSlash(p)
	if runtime.GOOS == "windows" {
		result = strings.ToLower(result)
	}
	return result
}

func padTwo(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func padFour(n int) string {
	s := itoa(n)
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
