package memory

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ────────────────────────────────────────────────────────────────────────────
// Memory file detection — aligned with claude-code-main
// src/utils/memoryFileDetection.ts
// ────────────────────────────────────────────────────────────────────────────

// FileKind classifies detected memory-related files.
type FileKind string

const (
	FileKindSessionMemory FileKind = "session_memory"
	FileKindAutoMemory    FileKind = "auto_memory"
	FileKindAgentMemory   FileKind = "agent_memory"
	FileKindTeamMemory    FileKind = "team_memory"
	FileKindClaudeConfig  FileKind = "claude_config"
	FileKindUnknown       FileKind = "unknown"
)

// DetectedFile holds info about a detected memory-related file.
type DetectedFile struct {
	Path string
	Kind FileKind
}

// DetectMemoryFile classifies an absolute file path as a memory-related file.
func DetectMemoryFile(absPath, projectRoot string) DetectedFile {
	normalised := filepath.Clean(absPath)

	// Check session memory directories
	if isUnderSessionDir(normalised) {
		return DetectedFile{Path: normalised, Kind: FileKindSessionMemory}
	}

	// Check auto-memory directory
	autoDir := GetAutoMemPath(projectRoot)
	if isPathUnderDirDetect(normalised, autoDir) {
		// Further check: is it in the team subdirectory?
		teamDir := filepath.Join(strings.TrimRight(autoDir, string(filepath.Separator)), "team")
		if isPathUnderDirDetect(normalised, teamDir+string(filepath.Separator)) {
			return DetectedFile{Path: normalised, Kind: FileKindTeamMemory}
		}
		return DetectedFile{Path: normalised, Kind: FileKindAutoMemory}
	}

	// Check agent memory (.claude/memory in project)
	agentDir := filepath.Join(projectRoot, ".claude", "memory")
	if isPathUnderDirDetect(normalised, agentDir+string(filepath.Separator)) {
		return DetectedFile{Path: normalised, Kind: FileKindAgentMemory}
	}

	// Check if it's a CLAUDE.md or claude.md
	base := filepath.Base(normalised)
	if strings.EqualFold(base, "CLAUDE.md") {
		return DetectedFile{Path: normalised, Kind: FileKindAgentMemory}
	}

	// Check general .claude config dir
	configHome := GetClaudeConfigHomeDir()
	if isPathUnderDirDetect(normalised, configHome+string(filepath.Separator)) {
		return DetectedFile{Path: normalised, Kind: FileKindClaudeConfig}
	}

	return DetectedFile{Path: normalised, Kind: FileKindUnknown}
}

// IsMemoryRelatedPath returns true if the path is related to any memory system.
func IsMemoryRelatedPath(absPath, projectRoot string) bool {
	d := DetectMemoryFile(absPath, projectRoot)
	return d.Kind != FileKindUnknown
}

// IsMemoryTargetCommand checks if a shell command targets memory files.
// This is used to detect when the user is modifying memory outside of the
// dedicated memory tools.
func IsMemoryTargetCommand(cmd, projectRoot string) bool {
	cmdLower := strings.ToLower(cmd)
	// Check for common file-manipulation commands targeting memory paths
	memoryIndicators := []string{
		"memory.md",
		".claude/memory",
		"/memory/",
		"claude.md",
	}
	for _, ind := range memoryIndicators {
		if strings.Contains(cmdLower, strings.ToLower(ind)) {
			return true
		}
	}

	autoDir := GetAutoMemPath(projectRoot)
	if autoDir != "" {
		normalised := toComparableDetect(autoDir)
		if strings.Contains(toComparableDetect(cmd), normalised) {
			return true
		}
	}

	return false
}

// ListMemoryFiles returns all .md files in the auto-memory directory.
func ListMemoryFiles(projectRoot string) ([]string, error) {
	autoDir := GetAutoMemPath(projectRoot)
	trimmed := strings.TrimRight(autoDir, string(filepath.Separator))

	entries, err := os.ReadDir(trimmed)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		files = append(files, filepath.Join(trimmed, e.Name()))
	}
	return files, nil
}

// ── Helpers ────────────────────────────────────────────────────────────────

func isUnderSessionDir(path string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	sessionDir := filepath.Join(home, ".claude", "sessions")
	return isPathUnderDirDetect(path, sessionDir+string(filepath.Separator))
}

func isPathUnderDirDetect(candidate, dir string) bool {
	c := toComparableDetect(candidate)
	d := toComparableDetect(dir)
	if !strings.HasSuffix(d, "/") {
		d += "/"
	}
	return strings.HasPrefix(c+"/", d) || c == strings.TrimRight(d, "/")
}

func toComparableDetect(p string) string {
	result := filepath.ToSlash(p)
	if runtime.GOOS == "windows" {
		result = strings.ToLower(result)
	}
	return result
}
