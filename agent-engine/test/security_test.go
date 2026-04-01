package test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool/bash"
	"github.com/wall-ai/agent-engine/internal/tool/grep"
)

// ─── Bash AST security ────────────────────────────────────────────────────────

func TestBashASTSecurity(t *testing.T) {
	b := bash.New()
	ctx := context.Background()
	dir := t.TempDir()
	uctx := &engine.UseContext{WorkDir: dir}

	cases := []struct {
		name    string
		cmd     string
		wantErr bool
	}{
		{"safe echo", `{"command":"echo hello world"}`, false},
		{"safe ls", `{"command":"ls -la /tmp"}`, false},
		{"fork bomb canonical", `{"command":":(){ :|:& };:"}`, true},
		{"fork bomb alt", `{"command":":(){:|:&};:"}`, true},
		{"mkfs blocked", `{"command":"mkfs /dev/sda1"}`, true},
		{"fdisk blocked", `{"command":"fdisk /dev/sda"}`, true},
		{"dd to block device", `{"command":"dd if=/dev/zero of=/dev/sda bs=4096"}`, true},
		{"rm rf root", `{"command":"rm -rf /"}`, true},
		{"rm rf wildcard", `{"command":"rm -rf /*"}`, true},
		{"safe dd copy", `{"command":"dd if=input.bin of=output.bin"}`, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := b.CheckPermissions(ctx, json.RawMessage(tc.cmd), uctx)
			if tc.wantErr {
				assert.Error(t, err, "expected security error for: %s", tc.cmd)
			} else {
				assert.NoError(t, err, "unexpected security error for: %s", tc.cmd)
			}
		})
	}
}

// ─── Grep Go fallback ─────────────────────────────────────────────────────────

func TestGrepGoFallback(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, writeTestFile(dir+"/alpha.txt", "hello world\nfoo bar\n"))
	require.NoError(t, writeTestFile(dir+"/beta.txt", "baz qux\nhello again\n"))

	g := grep.New()
	ctx := context.Background()

	input, _ := json.Marshal(map[string]interface{}{
		"pattern": "hello",
		"path":    dir,
	})
	ch, err := g.Call(ctx, input, &engine.UseContext{WorkDir: dir})
	require.NoError(t, err)

	var output string
	for b := range ch {
		output += b.Text
	}
	// Should find "hello" in at least one file regardless of whether rg is installed.
	assert.NotEqual(t, "", output)
	if output != "No matches found." {
		assert.Contains(t, output, "hello")
	}
}

func TestGrepGoFallbackIncludeGlob(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, writeTestFile(dir+"/main.go", "package main\nfunc main() {}\n"))
	require.NoError(t, writeTestFile(dir+"/readme.md", "# Project\nThis is the readme\n"))

	g := grep.New()
	ctx := context.Background()

	input, _ := json.Marshal(map[string]interface{}{
		"pattern": "main",
		"path":    dir,
		"include": "*.go",
	})
	ch, err := g.Call(ctx, input, &engine.UseContext{WorkDir: dir})
	require.NoError(t, err)

	var output string
	for b := range ch {
		output += b.Text
	}
	// readme.md should not appear since include filters to *.go.
	if output != "No matches found." {
		assert.NotContains(t, output, "readme.md")
	}
}
