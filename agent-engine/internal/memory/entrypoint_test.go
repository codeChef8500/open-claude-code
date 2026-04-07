package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTruncateEntrypointContent(t *testing.T) {
	t.Run("short content", func(t *testing.T) {
		result := TruncateEntrypointContent("line 1\nline 2\nline 3")
		if result.WasLineTruncated || result.WasByteTruncated {
			t.Error("short content should not be truncated")
		}
		if result.LineCount != 3 {
			t.Errorf("LineCount = %d, want 3", result.LineCount)
		}
	})

	t.Run("line truncation", func(t *testing.T) {
		lines := make([]string, MaxEntrypointLines+50)
		for i := range lines {
			lines[i] = "short line"
		}
		raw := strings.Join(lines, "\n")
		result := TruncateEntrypointContent(raw)
		if !result.WasLineTruncated {
			t.Error("expected line truncation")
		}
		// Content should have warning
		if !strings.Contains(result.Content, "WARNING") {
			t.Error("truncated content should have WARNING")
		}
		// Resulting lines (excluding warning) should be ≤ MaxEntrypointLines
		contentLines := strings.Split(strings.Split(result.Content, "\n\n> WARNING")[0], "\n")
		if len(contentLines) > MaxEntrypointLines {
			t.Errorf("truncated content has %d lines, want ≤ %d", len(contentLines), MaxEntrypointLines)
		}
	})

	t.Run("byte truncation", func(t *testing.T) {
		// Few lines but very long
		longLine := strings.Repeat("x", MaxEntrypointBytes+1000)
		result := TruncateEntrypointContent(longLine)
		if !result.WasByteTruncated {
			t.Error("expected byte truncation")
		}
		if !strings.Contains(result.Content, "WARNING") {
			t.Error("truncated content should have WARNING")
		}
	})
}

func TestReadWriteEntrypoint(t *testing.T) {
	tmpDir := t.TempDir()
	ClearAutoMemPathCache()
	t.Setenv(envMemoryPathOverride, tmpDir+string(filepath.Separator))

	// Initially empty
	content := ReadEntrypoint("/any")
	if content != "" {
		t.Errorf("ReadEntrypoint on empty dir = %q, want empty", content)
	}

	// Write
	err := WriteEntrypoint("/any", "# My Memory\n- item 1\n")
	if err != nil {
		t.Fatalf("WriteEntrypoint: %v", err)
	}

	// Read back
	content = ReadEntrypoint("/any")
	if !strings.Contains(content, "My Memory") {
		t.Errorf("ReadEntrypoint = %q, want to contain 'My Memory'", content)
	}
}

func TestEnsureMemoryDirExists(t *testing.T) {
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "a", "b", "c")
	if err := EnsureMemoryDirExists(newDir); err != nil {
		t.Fatalf("EnsureMemoryDirExists: %v", err)
	}
	info, err := os.Stat(newDir)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestBuildMemoryLines(t *testing.T) {
	lines := BuildMemoryLines("auto memory", "/tmp/memory/", nil, false)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "# auto memory") {
		t.Error("missing header")
	}
	if !strings.Contains(joined, "MEMORY.md") {
		t.Error("missing entrypoint reference")
	}
	if !strings.Contains(joined, "How to save memories") {
		t.Error("missing how-to-save section")
	}
	if !strings.Contains(joined, "Step 2") {
		t.Error("missing Step 2 (index) when skipIndex=false")
	}
}

func TestBuildMemoryLinesSkipIndex(t *testing.T) {
	lines := BuildMemoryLines("auto memory", "/tmp/memory/", nil, true)
	joined := strings.Join(lines, "\n")

	if strings.Contains(joined, "Step 2") {
		t.Error("should not have Step 2 when skipIndex=true")
	}
}

func TestBuildMemoryPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	memDir := filepath.Join(tmpDir, "memory") + string(filepath.Separator)
	_ = os.MkdirAll(memDir, 0o700)

	// Without MEMORY.md
	prompt := BuildMemoryPrompt("auto memory", memDir, nil)
	if !strings.Contains(prompt, "currently empty") {
		t.Error("should show empty message when no MEMORY.md")
	}

	// With MEMORY.md
	_ = os.WriteFile(filepath.Join(memDir, AutoMemEntrypointName),
		[]byte("- [Test](test.md) — a test memory\n"), 0o600)
	prompt = BuildMemoryPrompt("auto memory", memDir, nil)
	if !strings.Contains(prompt, "Test") {
		t.Error("should include MEMORY.md content")
	}
}

func TestBuildAssistantDailyLogPrompt(t *testing.T) {
	prompt := BuildAssistantDailyLogPrompt("/tmp/memory/", false)
	if !strings.Contains(prompt, "daily log file") {
		t.Error("missing daily log instruction")
	}
	if !strings.Contains(prompt, "YYYY-MM-DD.md") {
		t.Error("missing log path pattern")
	}
	if !strings.Contains(prompt, AutoMemEntrypointName) {
		t.Error("missing entrypoint section")
	}
}

func TestBuildAssistantDailyLogPromptSkipIndex(t *testing.T) {
	prompt := BuildAssistantDailyLogPrompt("/tmp/memory/", true)
	if strings.Contains(prompt, "distilled index") {
		t.Error("should skip index section when skipIndex=true")
	}
}

func TestBuildSearchingPastContextSection(t *testing.T) {
	lines := BuildSearchingPastContextSection("/tmp/memory/")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Searching past context") {
		t.Error("missing section header")
	}
	if !strings.Contains(joined, "/tmp/memory/") {
		t.Error("missing memory dir in search examples")
	}
}

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		bytes int
		want  string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{25000, "24.4 KB"},
		{1048576, "1.0 MB"},
	}
	for _, tt := range tests {
		got := formatFileSize(tt.bytes)
		if got != tt.want {
			t.Errorf("formatFileSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestLoadMemoryPrompt(t *testing.T) {
	ClearAutoMemPathCache()
	tmpDir := t.TempDir()
	t.Setenv(envMemoryPathOverride, tmpDir+string(filepath.Separator))
	t.Setenv(envDisableAutoMemory, "")

	prompt := LoadMemoryPrompt("/any", false, nil)
	if prompt == "" {
		t.Error("LoadMemoryPrompt should return non-empty when enabled")
	}
	if !strings.Contains(prompt, "auto memory") {
		t.Error("missing auto memory header")
	}
}

func TestLoadMemoryPromptDisabled(t *testing.T) {
	t.Setenv(envDisableAutoMemory, "1")
	prompt := LoadMemoryPrompt("/any", false, nil)
	if prompt != "" {
		t.Errorf("LoadMemoryPrompt should return empty when disabled, got %q", prompt)
	}
}
