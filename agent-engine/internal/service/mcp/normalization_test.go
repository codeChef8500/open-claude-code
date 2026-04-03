package mcp

import (
	"testing"
)

func TestNormalizeNameForMCP(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"my-server", "my-server"},
		{"my_server", "my_server"},
		{"has spaces", "has_spaces"},
		{"special!@#chars", "special___chars"},
		{"UPPER", "UPPER"},
		{"a1_b2-c3", "a1_b2-c3"},
	}
	for _, tt := range tests {
		got := NormalizeNameForMCP(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeNameForMCP(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeNameForMCP_ClaudeAIPrefix(t *testing.T) {
	// claude.ai prefixed names get extra normalization.
	input := "claude.ai my__server"
	got := NormalizeNameForMCP(input)
	// Should collapse underscores and trim leading/trailing.
	if got == "" {
		t.Error("expected non-empty result")
	}
	// Should not contain consecutive underscores after normalization.
	for i := 0; i < len(got)-1; i++ {
		if got[i] == '_' && got[i+1] == '_' {
			t.Errorf("consecutive underscores found in %q", got)
			break
		}
	}
}

func TestBuildMcpToolName(t *testing.T) {
	got := BuildMcpToolName("my-server", "read_file")
	want := "mcp__my-server__read_file"
	if got != want {
		t.Errorf("BuildMcpToolName = %q, want %q", got, want)
	}
}

func TestMcpInfoFromString(t *testing.T) {
	tests := []struct {
		input      string
		wantNil    bool
		wantServer string
		wantTool   string
	}{
		{"mcp__myserver__read_file", false, "myserver", "read_file"},
		{"mcp__srv", false, "srv", ""},
		{"mcp____tool", true, "", ""}, // empty server -> nil
		{"not_mcp__srv__tool", true, "", ""},
		{"mcp", true, "", ""},
		{"", true, "", ""},
	}
	for _, tt := range tests {
		info := McpInfoFromString(tt.input)
		if tt.wantNil {
			if info != nil {
				t.Errorf("McpInfoFromString(%q): expected nil, got %+v", tt.input, info)
			}
			continue
		}
		if info == nil {
			t.Errorf("McpInfoFromString(%q): expected non-nil", tt.input)
			continue
		}
		if info.ServerName != tt.wantServer {
			t.Errorf("McpInfoFromString(%q).ServerName = %q, want %q", tt.input, info.ServerName, tt.wantServer)
		}
		if info.ToolName != tt.wantTool {
			t.Errorf("McpInfoFromString(%q).ToolName = %q, want %q", tt.input, info.ToolName, tt.wantTool)
		}
	}
}

func TestGetMcpPrefix(t *testing.T) {
	prefix := GetMcpPrefix("my-server")
	want := "mcp__my-server__"
	if prefix != want {
		t.Errorf("GetMcpPrefix = %q, want %q", prefix, want)
	}
}

func TestGetMcpDisplayName(t *testing.T) {
	full := "mcp__myserver__read_file"
	got := GetMcpDisplayName(full, "myserver")
	if got != "read_file" {
		t.Errorf("GetMcpDisplayName = %q, want 'read_file'", got)
	}
}

func TestGetMcpDisplayName_NoMatch(t *testing.T) {
	got := GetMcpDisplayName("some_tool", "myserver")
	if got != "some_tool" {
		t.Errorf("expected unchanged name, got %q", got)
	}
}

func TestGetToolNameForPermissionCheck_MCP(t *testing.T) {
	info := &McpToolInfo{ServerName: "srv", ToolName: "tool"}
	got := GetToolNameForPermissionCheck("tool", info)
	want := "mcp__srv__tool"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestGetToolNameForPermissionCheck_Builtin(t *testing.T) {
	got := GetToolNameForPermissionCheck("Read", nil)
	if got != "Read" {
		t.Errorf("expected 'Read', got %q", got)
	}
}

func TestBuildAndParse_RoundTrip(t *testing.T) {
	server := "test-server"
	tool := "my_tool"
	fullName := BuildMcpToolName(server, tool)

	info := McpInfoFromString(fullName)
	if info == nil {
		t.Fatal("expected non-nil info from round-trip")
	}
	if info.ServerName != server {
		t.Errorf("server: expected %q, got %q", server, info.ServerName)
	}
	if info.ToolName != tool {
		t.Errorf("tool: expected %q, got %q", tool, info.ToolName)
	}
}
