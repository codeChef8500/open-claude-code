package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool/bash"
	"github.com/wall-ai/agent-engine/internal/tool/fileedit"
	"github.com/wall-ai/agent-engine/internal/tool/fileread"
	"github.com/wall-ai/agent-engine/internal/tool/filewrite"
	"github.com/wall-ai/agent-engine/internal/tool/glob"
	"github.com/wall-ai/agent-engine/internal/tool/grep"
	"github.com/wall-ai/agent-engine/internal/tool/powershell"
	"github.com/wall-ai/agent-engine/internal/toolset"
	"github.com/wall-ai/agent-engine/pkg/sdk"
)

// ═══════════════════════════════════════════════════════════════════════════
// E2E Test 1: Tool Registration — ensure all tools are present
// ═══════════════════════════════════════════════════════════════════════════

func TestE2E_ToolRegistration(t *testing.T) {
	tools := toolset.DefaultTools(nil)
	require.NotEmpty(t, tools, "DefaultTools should return a non-empty list")

	names := make(map[string]bool)
	for _, tool := range tools {
		name := tool.Name()
		assert.NotEmpty(t, name, "every tool must have a name")
		assert.False(t, names[name], "duplicate tool name: %s", name)
		names[name] = true
	}

	// Verify critical tools are registered.
	critical := []string{"Read", "Edit", "Write", "Grep", "Glob", "Bash", "Task", "TodoWrite"}
	for _, c := range critical {
		assert.True(t, names[c], "critical tool %q must be registered", c)
	}

	// On Windows, PowerShell should also be registered.
	if runtime.GOOS == "windows" {
		assert.True(t, names["PowerShell"], "PowerShell tool must be registered on Windows")
	}

	t.Logf("Registered %d tools: %v", len(tools), sortedKeys(names))
}

// ═══════════════════════════════════════════════════════════════════════════
// E2E Test 2: Tool InputSchema — ensure schemas are valid JSON with required fields
// ═══════════════════════════════════════════════════════════════════════════

func TestE2E_ToolSchemas(t *testing.T) {
	tools := toolset.DefaultTools(nil)
	for _, tool := range tools {
		t.Run(tool.Name(), func(t *testing.T) {
			schema := tool.InputSchema()
			require.NotNil(t, schema, "%s: InputSchema must not be nil", tool.Name())

			var parsed map[string]interface{}
			err := json.Unmarshal(schema, &parsed)
			require.NoError(t, err, "%s: InputSchema must be valid JSON", tool.Name())

			// Every tool schema should have "type":"object" and "properties".
			typ, ok := parsed["type"]
			assert.True(t, ok, "%s: schema missing 'type'", tool.Name())
			assert.Equal(t, "object", typ, "%s: schema type should be 'object'", tool.Name())

			props, ok := parsed["properties"]
			assert.True(t, ok, "%s: schema missing 'properties'", tool.Name())
			assert.NotNil(t, props, "%s: properties should not be nil", tool.Name())
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// E2E Test 3: Individual tool execution — Bash / PowerShell
// ═══════════════════════════════════════════════════════════════════════════

func TestE2E_BashToolExecution(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	var cmd string
	if runtime.GOOS == "windows" {
		cmd = `echo HELLO_E2E`
	} else {
		cmd = `echo HELLO_E2E`
	}

	input, _ := json.Marshal(map[string]interface{}{"command": cmd})
	b := bash.New()

	ch, err := b.Call(ctx, input, &engine.UseContext{WorkDir: dir})
	require.NoError(t, err)

	var output string
	for block := range ch {
		output += block.Text
	}
	assert.Contains(t, output, "HELLO_E2E", "bash echo should produce expected output")
	t.Logf("Bash output: %s", strings.TrimSpace(output))
}

func TestE2E_PowerShellToolExecution(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PowerShell test only runs on Windows")
	}
	dir := t.TempDir()
	ctx := context.Background()

	input, _ := json.Marshal(map[string]interface{}{"command": "Write-Output 'PS_E2E_OK'"})
	ps := powershell.New()

	ch, err := ps.Call(ctx, input, &engine.UseContext{WorkDir: dir})
	require.NoError(t, err)

	var output string
	for block := range ch {
		output += block.Text
	}
	assert.Contains(t, output, "PS_E2E_OK", "PowerShell should produce expected output")
	t.Logf("PowerShell output: %s", strings.TrimSpace(output))
}

// ═══════════════════════════════════════════════════════════════════════════
// E2E Test 4: File operation chain — Write → Read → Edit → Read → Verify
// ═══════════════════════════════════════════════════════════════════════════

func TestE2E_FileOpsChain(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	target := filepath.Join(dir, "e2e_test.txt")

	// Step 1: Write file
	fw := filewrite.New()
	writeInput, _ := json.Marshal(map[string]interface{}{
		"file_path": target,
		"content":   "line1\nline2\nline3\n",
	})
	ch, err := fw.Call(ctx, writeInput, &engine.UseContext{WorkDir: dir})
	require.NoError(t, err)
	drainBlocks(ch)

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "line1\nline2\nline3\n", string(content))

	// Step 2: Read file (populates cache for edit)
	fr := fileread.New()
	readInput, _ := json.Marshal(map[string]interface{}{"file_path": target})
	ch, err = fr.Call(ctx, readInput, &engine.UseContext{WorkDir: dir})
	require.NoError(t, err)
	readOutput := collectBlockText(ch)
	assert.Contains(t, readOutput, "line1")
	assert.Contains(t, readOutput, "line3")

	// Step 3: Edit file (replace line2 with EDITED)
	fe := fileedit.New()
	editInput, _ := json.Marshal(map[string]interface{}{
		"file_path":  target,
		"old_string": "line2",
		"new_string": "EDITED",
	})
	ch, err = fe.Call(ctx, editInput, &engine.UseContext{WorkDir: dir})
	require.NoError(t, err)
	editOutput := collectBlockText(ch)
	t.Logf("Edit result: %s", editOutput)

	// Step 4: Verify on disk
	content, err = os.ReadFile(target)
	require.NoError(t, err)
	assert.Contains(t, string(content), "EDITED")
	assert.NotContains(t, string(content), "line2")
	t.Logf("Final file content: %s", string(content))
}

// ═══════════════════════════════════════════════════════════════════════════
// E2E Test 5: Grep + Glob tools
// ═══════════════════════════════════════════════════════════════════════════

func TestE2E_SearchTools(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Create test files.
	require.NoError(t, writeTestFile(filepath.Join(dir, "a.go"), "package main\nfunc hello() {}"))
	require.NoError(t, writeTestFile(filepath.Join(dir, "b.go"), "package main\nfunc world() {}"))
	require.NoError(t, writeTestFile(filepath.Join(dir, "c.txt"), "not a go file"))
	require.NoError(t, writeTestFile(filepath.Join(dir, "sub", "d.go"), "package sub\nfunc deep() {}"))

	// Glob: *.go at root
	gl := glob.New()
	globInput, _ := json.Marshal(map[string]interface{}{"pattern": "*.go", "path": dir})
	ch, err := gl.Call(ctx, globInput, &engine.UseContext{WorkDir: dir})
	require.NoError(t, err)
	globOutput := collectBlockText(ch)
	assert.Contains(t, globOutput, "a.go")
	assert.Contains(t, globOutput, "b.go")
	assert.NotContains(t, globOutput, "c.txt")
	t.Logf("Glob result: %s", globOutput)

	// Glob: recursive **/*.go
	globInput2, _ := json.Marshal(map[string]interface{}{"pattern": "**/*.go", "path": dir})
	ch, err = gl.Call(ctx, globInput2, &engine.UseContext{WorkDir: dir})
	require.NoError(t, err)
	globOutput2 := collectBlockText(ch)
	assert.Contains(t, globOutput2, "d.go")

	// Grep: search for "func"
	gr := grep.New()
	grepInput, _ := json.Marshal(map[string]interface{}{"pattern": "func", "path": dir})
	ch, err = gr.Call(ctx, grepInput, &engine.UseContext{WorkDir: dir})
	require.NoError(t, err)
	grepOutput := collectBlockText(ch)
	// Should find matches in .go files (not in c.txt).
	t.Logf("Grep result: %s", grepOutput)
}

// ═══════════════════════════════════════════════════════════════════════════
// E2E Test 6: Mock Engine — full query loop with tool calls
// ═══════════════════════════════════════════════════════════════════════════

// mockToolCallProvider simulates an LLM that requests a tool call, then responds with text.
type mockToolCallProvider struct {
	callCount int
	toolName  string
	toolInput map[string]interface{}
	finalText string
}

func (m *mockToolCallProvider) Name() string { return "mock_tool" }

func (m *mockToolCallProvider) CallModel(_ context.Context, params engine.CallParams) (<-chan *engine.StreamEvent, error) {
	m.callCount++
	ch := make(chan *engine.StreamEvent, 16)
	go func() {
		defer close(ch)
		if m.callCount == 1 && m.toolName != "" {
			// First call: request a tool use.
			inputBytes, _ := json.Marshal(m.toolInput)
			var inputMap interface{}
			_ = json.Unmarshal(inputBytes, &inputMap)
			ch <- &engine.StreamEvent{
				Type:      engine.EventToolUse,
				ToolID:    "tool_001",
				ToolName:  m.toolName,
				ToolInput: inputMap,
			}
			ch <- &engine.StreamEvent{Type: engine.EventUsage, Usage: &engine.UsageStats{
				InputTokens: 100, OutputTokens: 50,
			}}
			ch <- &engine.StreamEvent{Type: engine.EventDone}
		} else {
			// Second call (or if no tool): return final text.
			ch <- &engine.StreamEvent{Type: engine.EventTextDelta, Text: m.finalText}
			ch <- &engine.StreamEvent{Type: engine.EventUsage, Usage: &engine.UsageStats{
				InputTokens: 200, OutputTokens: 80,
			}}
			ch <- &engine.StreamEvent{Type: engine.EventDone}
		}
	}()
	return ch, nil
}

func TestE2E_EngineToolCallLoop(t *testing.T) {
	dir := t.TempDir()

	// Create a file for the Read tool to find.
	target := filepath.Join(dir, "test_data.txt")
	require.NoError(t, writeTestFile(target, "SENTINEL_VALUE_42"))

	prov := &mockToolCallProvider{
		toolName:  "Read",
		toolInput: map[string]interface{}{"file_path": target},
		finalText: "The file contains SENTINEL_VALUE_42.",
	}

	tools := toolset.DefaultTools(nil)
	eng, err := engine.New(engine.EngineConfig{
		Provider:  "mock",
		Model:     "test-model",
		MaxTokens: 4096,
		WorkDir:   dir,
		SessionID: "e2e-test-session",
	}, prov, tools)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ch := eng.SubmitMessage(ctx, engine.QueryParams{Text: "Read test_data.txt"})

	var textParts []string
	var toolsUsed []string
	var gotDone bool
	var gotToolResult bool

	for ev := range ch {
		switch ev.Type {
		case engine.EventTextDelta:
			textParts = append(textParts, ev.Text)
		case engine.EventToolUse:
			toolsUsed = append(toolsUsed, ev.ToolName)
			t.Logf("[tool_use] %s %v", ev.ToolName, ev.ToolInput)
		case engine.EventToolResult:
			gotToolResult = true
			t.Logf("[tool_result] tool=%s error=%v", ev.ToolName, ev.IsError)
		case engine.EventDone:
			gotDone = true
		case engine.EventError:
			t.Fatalf("unexpected error: %s", ev.Error)
		case engine.EventSystemMessage:
			t.Logf("[system] %s", ev.Text)
		}
	}

	assert.True(t, gotDone, "expected EventDone")
	assert.Contains(t, toolsUsed, "Read", "expected Read tool to be called")
	assert.True(t, gotToolResult, "expected tool result event")
	assert.Equal(t, 2, prov.callCount, "expected 2 provider calls (1 tool + 1 final)")
	fullText := strings.Join(textParts, "")
	assert.Contains(t, fullText, "SENTINEL_VALUE_42", "final text should reference file content")
	t.Logf("Full text: %s", fullText)
}

// ═══════════════════════════════════════════════════════════════════════════
// E2E Test 7: Mock Engine — multi-turn conversation with context
// ═══════════════════════════════════════════════════════════════════════════

func TestE2E_EngineMultiTurn(t *testing.T) {
	dir := t.TempDir()
	prov := &mockProvider{response: "I remember what you said."}

	tools := toolset.DefaultTools(nil)
	eng, err := engine.New(engine.EngineConfig{
		Provider:  "mock",
		Model:     "test-model",
		MaxTokens: 4096,
		WorkDir:   dir,
		SessionID: "e2e-multiturn",
	}, prov, tools)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Turn 1
	ch1 := eng.SubmitMessage(ctx, engine.QueryParams{Text: "My secret code is ABCXYZ."})
	for range ch1 {
	}

	// Turn 2
	prov.response = "Your secret code is ABCXYZ."
	ch2 := eng.SubmitMessage(ctx, engine.QueryParams{Text: "What was my secret code?"})
	var turn2Text string
	for ev := range ch2 {
		if ev.Type == engine.EventTextDelta {
			turn2Text += ev.Text
		}
	}
	assert.Contains(t, turn2Text, "ABCXYZ", "engine should maintain conversation history across turns")
	t.Logf("Turn 2 response: %s", turn2Text)
}

// ═══════════════════════════════════════════════════════════════════════════
// E2E Test 8: Live LLM — simple chat (skipped without API key)
// ═══════════════════════════════════════════════════════════════════════════

func TestE2E_LiveSimpleChat(t *testing.T) {
	apiKey, baseURL, provType, model := liveEnvConfig()
	if apiKey == "" {
		t.Skip("No API key — skipping live E2E test")
	}

	eng, err := sdk.New(
		sdk.WithProvider(provType),
		sdk.WithAPIKey(apiKey),
		sdk.WithModel(model),
		sdk.WithBaseURL(baseURL),
		sdk.WithWorkDir(t.TempDir()),
		sdk.WithMaxTokens(256),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	r := collectTurn(t, eng.SubmitMessage(ctx, "Reply with exactly: E2E_LIVE_OK"))
	assert.True(t, r.GotDone, "expected EventDone")
	assert.NotEmpty(t, r.Text, "expected non-empty response")
	t.Logf("Live response: %s", r.Text)
}

// ═══════════════════════════════════════════════════════════════════════════
// E2E Test 9: Live LLM — tool call with file read (skipped without API key)
// ═══════════════════════════════════════════════════════════════════════════

func TestE2E_LiveToolCall(t *testing.T) {
	apiKey, baseURL, provType, model := liveEnvConfig()
	if apiKey == "" {
		t.Skip("No API key — skipping live E2E test")
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "secret.txt")
	require.NoError(t, writeTestFile(target, "THE_SECRET_IS_BANANA"))

	eng, err := sdk.New(
		sdk.WithProvider(provType),
		sdk.WithAPIKey(apiKey),
		sdk.WithModel(model),
		sdk.WithBaseURL(baseURL),
		sdk.WithWorkDir(dir),
		sdk.WithMaxTokens(1024),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	r := collectTurn(t, eng.SubmitMessage(ctx,
		"Read the file "+target+" using the Read tool and tell me the secret."))
	assert.True(t, r.GotDone)
	assert.True(t, containsTool(r.ToolsUsed, "Read") || containsTool(r.ToolsUsed, "Bash"),
		"expected Read or Bash tool, got %v", r.ToolsUsed)
	t.Logf("Live tool call response: %s", r.Text)
	t.Logf("Tools used: %v", r.ToolsUsed)
}

// ═══════════════════════════════════════════════════════════════════════════
// E2E Test 10: Live LLM — bash/powershell execution (skipped without API key)
// ═══════════════════════════════════════════════════════════════════════════

func TestE2E_LiveBashExecution(t *testing.T) {
	apiKey, baseURL, provType, model := liveEnvConfig()
	if apiKey == "" {
		t.Skip("No API key — skipping live E2E test")
	}

	dir := t.TempDir()
	eng, err := sdk.New(
		sdk.WithProvider(provType),
		sdk.WithAPIKey(apiKey),
		sdk.WithModel(model),
		sdk.WithBaseURL(baseURL),
		sdk.WithWorkDir(dir),
		sdk.WithMaxTokens(1024),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	prompt := "Run this exact command using the Bash tool: echo E2E_BASH_TEST_OK"
	if runtime.GOOS == "windows" {
		prompt = "Run this exact command using the Bash tool: echo E2E_BASH_TEST_OK"
	}

	r := collectTurn(t, eng.SubmitMessage(ctx, prompt))
	assert.True(t, r.GotDone)
	assert.NotEmpty(t, r.ToolsUsed, "expected at least one tool call")
	t.Logf("Live bash response: %s", r.Text)
	t.Logf("Tools used: %v", r.ToolsUsed)
}

// ═══════════════════════════════════════════════════════════════════════════
// Helpers
// ═══════════════════════════════════════════════════════════════════════════

func drainBlocks(ch <-chan *engine.ContentBlock) {
	for range ch {
	}
}

func collectBlockText(ch <-chan *engine.ContentBlock) string {
	var sb strings.Builder
	for b := range ch {
		sb.WriteString(b.Text)
	}
	return sb.String()
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Simple insertion sort for test output.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
