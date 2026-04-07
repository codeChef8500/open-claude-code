package teammem

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/wall-ai/agent-engine/internal/memory"
)

func TestIsTeamMemoryEnabled(t *testing.T) {
	t.Setenv(envTeamMemEnabled, "")
	t.Setenv("CLAUDE_CODE_DISABLE_AUTO_MEMORY", "")
	if IsTeamMemoryEnabled() {
		t.Error("should be disabled by default")
	}

	t.Setenv(envTeamMemEnabled, "1")
	if !IsTeamMemoryEnabled() {
		t.Error("should be enabled when env=1")
	}

	// Disabled when auto-memory is off
	t.Setenv("CLAUDE_CODE_DISABLE_AUTO_MEMORY", "1")
	if IsTeamMemoryEnabled() {
		t.Error("should be disabled when auto-memory is off")
	}
}

func TestGetTeamMemPath(t *testing.T) {
	memory.ClearAutoMemPathCache()
	t.Setenv("CLAUDE_COWORK_MEMORY_PATH_OVERRIDE", "")
	t.Setenv("CLAUDE_CODE_REMOTE_MEMORY_DIR", "")
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	project := "/home/user/proj"
	if runtime.GOOS == "windows" {
		project = `C:\Users\user\proj`
	}

	path := GetTeamMemPath(project)
	if !strings.Contains(path, TeamMemDirName) {
		t.Errorf("GetTeamMemPath should contain %q: %q", TeamMemDirName, path)
	}
	if !strings.HasSuffix(path, string(filepath.Separator)) {
		t.Errorf("GetTeamMemPath should end with separator: %q", path)
	}
}

func TestValidateTeamMemKey(t *testing.T) {
	tests := []struct {
		key     string
		wantErr bool
	}{
		{"valid-key.md", false},
		{"my_memory", false},
		{"", true},
		{strings.Repeat("a", MaxTeamMemKeyLength+1), true},
		{"../escape", true},
		{"/absolute", true},
		{"with/slash", true},
		{"with\\backslash", true},
		{"null\x00byte", true},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			errMsg := ValidateTeamMemKey(tt.key)
			if tt.wantErr && errMsg == "" {
				t.Errorf("ValidateTeamMemKey(%q) should return error", tt.key)
			}
			if !tt.wantErr && errMsg != "" {
				t.Errorf("ValidateTeamMemKey(%q) = %q, want valid", tt.key, errMsg)
			}
		})
	}

	if runtime.GOOS == "windows" {
		for _, name := range []string{"CON", "PRN", "AUX", "NUL", "COM1", "LPT1"} {
			if errMsg := ValidateTeamMemKey(name); errMsg == "" {
				t.Errorf("ValidateTeamMemKey(%q) should reject Windows reserved name", name)
			}
		}
	}
}

func TestSanitizeTeamMemKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal.md", "normal.md"},
		{"has spaces.md", "has_spaces.md"},
		{"special!@#$.md", "special.md"},
		{"", "_unnamed.md"},
		{"no-ext", "no-ext.md"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeTeamMemKey(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeTeamMemKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsTeamMemFile(t *testing.T) {
	memory.ClearAutoMemPathCache()
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_COWORK_MEMORY_PATH_OVERRIDE", tmpDir+string(filepath.Separator))

	teamDir := GetTeamMemPath("/any")
	_ = os.MkdirAll(strings.TrimRight(teamDir, string(filepath.Separator)), 0o700)

	testFile := filepath.Join(strings.TrimRight(teamDir, string(filepath.Separator)), "test.md")
	if !IsTeamMemFile(testFile, "/any") {
		t.Error("should match file inside team dir")
	}
	if IsTeamMemFile("/some/other/file.md", "/any") {
		t.Error("should not match unrelated file")
	}
}

func TestIsWindowsReserved(t *testing.T) {
	for _, name := range []string{"CON", "con", "PRN", "AUX", "NUL", "COM1", "LPT1", "con.txt"} {
		if !isWindowsReserved(name) {
			t.Errorf("isWindowsReserved(%q) should be true", name)
		}
	}
	for _, name := range []string{"normal", "file.txt", "console", "communication"} {
		if isWindowsReserved(name) {
			t.Errorf("isWindowsReserved(%q) should be false", name)
		}
	}
}
