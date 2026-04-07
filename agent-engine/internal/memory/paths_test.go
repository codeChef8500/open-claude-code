package memory

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetClaudeConfigHomeDir(t *testing.T) {
	// With env override
	t.Setenv(envClaudeConfigDir, "/tmp/test-config")
	got := GetClaudeConfigHomeDir()
	if got != "/tmp/test-config" {
		t.Errorf("GetClaudeConfigHomeDir with env override = %q, want /tmp/test-config", got)
	}

	// Without env override — should use home dir
	t.Setenv(envClaudeConfigDir, "")
	got = GetClaudeConfigHomeDir()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".claude")
	if got != want {
		t.Errorf("GetClaudeConfigHomeDir default = %q, want %q", got, want)
	}
}

func TestGetMemoryBaseDir(t *testing.T) {
	t.Setenv(envRemoteMemoryDir, "/remote/memory")
	got := GetMemoryBaseDir()
	if got != "/remote/memory" {
		t.Errorf("GetMemoryBaseDir with env = %q, want /remote/memory", got)
	}

	t.Setenv(envRemoteMemoryDir, "")
	got = GetMemoryBaseDir()
	if got != GetClaudeConfigHomeDir() {
		t.Errorf("GetMemoryBaseDir default = %q, want %q", got, GetClaudeConfigHomeDir())
	}
}

func TestIsAutoMemoryEnabled(t *testing.T) {
	tests := []struct {
		name    string
		envs    map[string]string
		want    bool
	}{
		{
			name: "default enabled",
			envs: map[string]string{},
			want: true,
		},
		{
			name: "disabled via CLAUDE_CODE_DISABLE_AUTO_MEMORY=1",
			envs: map[string]string{envDisableAutoMemory: "1"},
			want: false,
		},
		{
			name: "disabled via CLAUDE_CODE_DISABLE_AUTO_MEMORY=true",
			envs: map[string]string{envDisableAutoMemory: "true"},
			want: false,
		},
		{
			name: "explicitly enabled via CLAUDE_CODE_DISABLE_AUTO_MEMORY=0",
			envs: map[string]string{envDisableAutoMemory: "0"},
			want: true,
		},
		{
			name: "disabled via SIMPLE mode",
			envs: map[string]string{envSimple: "1"},
			want: false,
		},
		{
			name: "disabled via REMOTE without memory dir",
			envs: map[string]string{envRemote: "1"},
			want: false,
		},
		{
			name: "enabled via REMOTE with memory dir",
			envs: map[string]string{
				envRemote:         "1",
				envRemoteMemoryDir: "/remote/memory",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all relevant env vars
			for _, key := range []string{envDisableAutoMemory, envSimple, envRemote, envRemoteMemoryDir} {
				t.Setenv(key, "")
			}
			for k, v := range tt.envs {
				t.Setenv(k, v)
			}
			got := IsAutoMemoryEnabled()
			if got != tt.want {
				t.Errorf("IsAutoMemoryEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateMemoryPath(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		expandTilde bool
		wantEmpty   bool
	}{
		{"empty", "", false, true},
		{"relative", "foo/bar", false, true},
		{"null byte", "/foo/\x00bar", false, true},
	}

	// Platform-specific tests
	if runtime.GOOS == "windows" {
		tests = append(tests,
			struct {
				name        string
				raw         string
				expandTilde bool
				wantEmpty   bool
			}{"drive root", "C:", false, true},
			struct {
				name        string
				raw         string
				expandTilde bool
				wantEmpty   bool
			}{"UNC path", `\\server\share`, false, true},
			struct {
				name        string
				raw         string
				expandTilde bool
				wantEmpty   bool
			}{"valid windows", `C:\Users\test\memory`, false, false},
		)
	} else {
		tests = append(tests,
			struct {
				name        string
				raw         string
				expandTilde bool
				wantEmpty   bool
			}{"root path", "/", false, true},
			struct {
				name        string
				raw         string
				expandTilde bool
				wantEmpty   bool
			}{"short path", "/a", false, true},
			struct {
				name        string
				raw         string
				expandTilde bool
				wantEmpty   bool
			}{"UNC slash", "//server/share", false, true},
			struct {
				name        string
				raw         string
				expandTilde bool
				wantEmpty   bool
			}{"valid unix", "/tmp/test-memory", false, false},
		)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateMemoryPath(tt.raw, tt.expandTilde)
			if tt.wantEmpty && got != "" {
				t.Errorf("validateMemoryPath(%q) = %q, want empty", tt.raw, got)
			}
			if !tt.wantEmpty && got == "" {
				t.Errorf("validateMemoryPath(%q) = empty, want non-empty", tt.raw)
			}
			if !tt.wantEmpty && got != "" {
				// Should have trailing separator
				sep := string(filepath.Separator)
				if !strings.HasSuffix(got, sep) {
					t.Errorf("validateMemoryPath(%q) = %q, missing trailing separator", tt.raw, got)
				}
			}
		})
	}
}

func TestSanitizePathForMemory(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "_empty"},
		{"/home/user/project", "home_user_project"},
		{"/tmp/test", "tmp_test"},
	}
	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			input string
			want  string
		}{`C:\Users\test\project`, "C_Users_test_project"})
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizePathForMemory(tt.input)
			if got != tt.want {
				t.Errorf("SanitizePathForMemory(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetAutoMemPath(t *testing.T) {
	ClearAutoMemPathCache()
	t.Setenv(envMemoryPathOverride, "")
	t.Setenv(envRemoteMemoryDir, "")
	t.Setenv(envClaudeConfigDir, "")

	projectRoot := "/home/user/myproject"
	if runtime.GOOS == "windows" {
		projectRoot = `C:\Users\user\myproject`
	}
	got := GetAutoMemPath(projectRoot)
	if got == "" {
		t.Fatal("GetAutoMemPath returned empty")
	}
	// Should contain projects/ and memory/
	if !strings.Contains(got, ProjectsDirName) {
		t.Errorf("GetAutoMemPath should contain %q: %q", ProjectsDirName, got)
	}
	if !strings.Contains(got, AutoMemDirName) {
		t.Errorf("GetAutoMemPath should contain %q: %q", AutoMemDirName, got)
	}
	// Should end with separator
	sep := string(filepath.Separator)
	if !strings.HasSuffix(got, sep) {
		t.Errorf("GetAutoMemPath should end with separator: %q", got)
	}

	// Test caching — same result on second call
	got2 := GetAutoMemPath(projectRoot)
	if got != got2 {
		t.Errorf("GetAutoMemPath not cached: %q != %q", got, got2)
	}
}

func TestGetAutoMemPathWithOverride(t *testing.T) {
	ClearAutoMemPathCache()
	if runtime.GOOS == "windows" {
		t.Setenv(envMemoryPathOverride, `C:\override\memory`)
	} else {
		t.Setenv(envMemoryPathOverride, "/override/memory")
	}
	got := GetAutoMemPath("/any/project")
	if runtime.GOOS == "windows" {
		if !strings.Contains(strings.ToLower(got), "override") {
			t.Errorf("GetAutoMemPath with override = %q, expected override path", got)
		}
	} else {
		want := "/override/memory/"
		if got != want {
			t.Errorf("GetAutoMemPath with override = %q, want %q", got, want)
		}
	}
}

func TestGetAutoMemEntrypoint(t *testing.T) {
	ClearAutoMemPathCache()
	t.Setenv(envMemoryPathOverride, "")
	ep := GetAutoMemEntrypoint("/tmp/project")
	if !strings.HasSuffix(ep, AutoMemEntrypointName) {
		t.Errorf("GetAutoMemEntrypoint should end with %q: %q", AutoMemEntrypointName, ep)
	}
}

func TestIsAutoMemPath(t *testing.T) {
	ClearAutoMemPathCache()
	t.Setenv(envMemoryPathOverride, "")
	project := "/home/user/myproject"
	if runtime.GOOS == "windows" {
		project = `C:\Users\user\myproject`
	}

	autoDir := GetAutoMemPath(project)
	testFile := filepath.Join(strings.TrimRight(autoDir, string(filepath.Separator)), "test.md")

	if !IsAutoMemPath(testFile, project) {
		t.Errorf("IsAutoMemPath should match file inside auto-mem dir: %q", testFile)
	}
	if IsAutoMemPath("/some/other/path.md", project) {
		t.Error("IsAutoMemPath should not match unrelated path")
	}
}

func TestToComparable(t *testing.T) {
	got := toComparable(`foo\bar/baz`)
	if strings.Contains(got, `\`) {
		t.Errorf("toComparable should use forward slashes: %q", got)
	}
}

func TestIsEnvTruthy(t *testing.T) {
	for _, val := range []string{"1", "true", "True", "TRUE", "yes", "YES"} {
		if !isEnvTruthy(val) {
			t.Errorf("isEnvTruthy(%q) = false, want true", val)
		}
	}
	for _, val := range []string{"", "0", "false", "no", "abc"} {
		if isEnvTruthy(val) {
			t.Errorf("isEnvTruthy(%q) = true, want false", val)
		}
	}
}

func TestIsEnvDefinedFalsy(t *testing.T) {
	for _, val := range []string{"0", "false", "False", "FALSE", "no"} {
		if !isEnvDefinedFalsy(val) {
			t.Errorf("isEnvDefinedFalsy(%q) = false, want true", val)
		}
	}
	if isEnvDefinedFalsy("") {
		t.Error("isEnvDefinedFalsy empty should be false")
	}
}

func TestPadTwo(t *testing.T) {
	if got := padTwo(1); got != "01" {
		t.Errorf("padTwo(1) = %q", got)
	}
	if got := padTwo(12); got != "12" {
		t.Errorf("padTwo(12) = %q", got)
	}
}
