package memory

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDetectMemoryFile(t *testing.T) {
	ClearAutoMemPathCache()
	t.Setenv("CLAUDE_COWORK_MEMORY_PATH_OVERRIDE", "")
	t.Setenv("CLAUDE_CODE_REMOTE_MEMORY_DIR", "")
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	project := "/home/user/proj"
	if runtime.GOOS == "windows" {
		project = `C:\Users\user\proj`
	}

	t.Run("CLAUDE.md is agent memory", func(t *testing.T) {
		path := filepath.Join(project, "CLAUDE.md")
		d := DetectMemoryFile(path, project)
		if d.Kind != FileKindAgentMemory {
			t.Errorf("CLAUDE.md should be agent memory, got %q", d.Kind)
		}
	})

	t.Run("agent memory dir", func(t *testing.T) {
		path := filepath.Join(project, ".claude", "memory", "test.md")
		d := DetectMemoryFile(path, project)
		if d.Kind != FileKindAgentMemory {
			t.Errorf(".claude/memory file should be agent memory, got %q", d.Kind)
		}
	})

	t.Run("unrelated file", func(t *testing.T) {
		path := filepath.Join(project, "src", "main.go")
		d := DetectMemoryFile(path, project)
		if d.Kind != FileKindUnknown {
			t.Errorf("src/main.go should be unknown, got %q", d.Kind)
		}
	})
}

func TestIsMemoryRelatedPath(t *testing.T) {
	project := "/tmp/project"
	if runtime.GOOS == "windows" {
		project = `C:\tmp\project`
	}

	if !IsMemoryRelatedPath(filepath.Join(project, "CLAUDE.md"), project) {
		t.Error("CLAUDE.md should be memory-related")
	}
	if IsMemoryRelatedPath(filepath.Join(project, "main.go"), project) {
		t.Error("main.go should not be memory-related")
	}
}

func TestIsMemoryTargetCommand(t *testing.T) {
	project := "/tmp/project"

	if !IsMemoryTargetCommand("cat MEMORY.md", project) {
		t.Error("should detect memory.md in command")
	}
	if !IsMemoryTargetCommand("ls .claude/memory/", project) {
		t.Error("should detect .claude/memory in command")
	}
	if IsMemoryTargetCommand("go build ./...", project) {
		t.Error("should not detect memory in go build")
	}
}
