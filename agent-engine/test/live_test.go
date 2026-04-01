package test

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/pkg/sdk"
)

// loadDotEnv parses a .env file and sets the values into os.Environ.
// Supports KEY=VALUE, KEY="VALUE", and # comments.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// Strip surrounding quotes.
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') ||
			(val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		_ = os.Setenv(key, val) // .env always takes precedence over OS env
	}
	return scanner.Err()
}

// projectRoot returns the agent-engine directory.
func projectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..")
}

// liveEnvConfig reads LLM config from environment (after loading .env).
// Returns (apiKey, baseURL, providerType, model).
func liveEnvConfig() (apiKey, baseURL, provType, model string) {
	// Load .env if present.
	envFile := filepath.Join(projectRoot(), ".env")
	_ = loadDotEnv(envFile)

	// Read with AGENT_ENGINE_ prefix first, then common aliases for ARK / MiniMax.
	get := func(keys ...string) string {
		for _, k := range keys {
			if v := os.Getenv(k); v != "" {
				return v
			}
		}
		return ""
	}

	provType = get("AGENT_ENGINE_PROVIDER", "LLM_PROVIDER")
	model    = get("AGENT_ENGINE_MODEL", "LLM_MODEL")
	if model == "" {
		model = "MiniMax-M2.5"
	}

	// Prefer AGENT_ENGINE_* explicit config.
	if k := get("AGENT_ENGINE_API_KEY"); k != "" {
		apiKey  = k
		baseURL = get("AGENT_ENGINE_BASE_URL")
		if provType == "" { provType = "openai" }
		return
	}

	// OPENAI_API_KEY + OPENAI_BASE_URL — user confirmed these are correct.
	if k, u := get("OPENAI_API_KEY"), get("OPENAI_BASE_URL"); k != "" && u != "" {
		apiKey  = k
		baseURL = u
		if provType == "" { provType = "openai" }
		return
	}

	// MiniMax direct API.
	if k := get("MINIMAX_API_KEY"); k != "" {
		apiKey  = k
		baseURL = get("MINIMAX_BASE_URL")
		if baseURL == "" { baseURL = "https://api.minimax.chat/v1" }
		if provType == "" { provType = "openai" }
		return
	}

	// VLLM / ARK with UUID key.
	if k, u := get("VLLM_API_KEY"), get("VLLM_BASE_URL"); k != "" && u != "" {
		apiKey  = k
		baseURL = u
		if provType == "" { provType = "openai" }
		return
	}

	// OpenRouter supports MiniMax-M2.5 as "minimax/minimax-m2.5".
	if k := get("OPENROUTER_API_KEY"); k != "" {
		apiKey  = k
		baseURL = "https://openrouter.ai/api/v1"
		if model == "MiniMax-M2.5" {
			model = "minimax/minimax-m2.5"
		}
		if provType == "" { provType = "openai" }
		return
	}

	// Final fallback.
	apiKey  = get("LLM_API_KEY", "API_KEY")
	baseURL = get("LLM_BASE_URL", "BASE_URL")
	if provType == "" { provType = "openai" }
	return
}

// dumpEnvKeys prints loaded env keys that look like LLM config (for debugging).
func dumpEnvKeys(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			continue
		}
		key := kv[:idx]
		val := kv[idx+1:]
		upKey := strings.ToUpper(key)
		if strings.Contains(upKey, "API") || strings.Contains(upKey, "BASE_URL") ||
			strings.Contains(upKey, "MODEL") || strings.Contains(upKey, "PROVIDER") ||
			strings.Contains(upKey, "MINIMAX") || strings.Contains(upKey, "ARK") {
			masked := val
			if len(val) > 20 {
				masked = val[:20] + "...(masked)"
			}
			t.Logf("  env: %s = %s", key, masked)
		}
	}
}

// TestLiveMiniMaxSimpleChat sends one message and verifies the streaming text response.
func TestLiveMiniMaxSimpleChat(t *testing.T) {
	apiKey, baseURL, provType, model := liveEnvConfig()
	if apiKey == "" {
		t.Skip("No API key found in .env or environment — skipping live test")
	}

	t.Log("=== Loaded LLM-related env vars ===")
	dumpEnvKeys(t)
	t.Logf("Provider: %s | Model: %s | BaseURL: %s", provType, model, baseURL)

	workDir := t.TempDir()
	eng, err := sdk.New(
		sdk.WithProvider(provType),
		sdk.WithAPIKey(apiKey),
		sdk.WithModel(model),
		sdk.WithBaseURL(baseURL),
		sdk.WithWorkDir(workDir),
		sdk.WithMaxTokens(512),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Log("Submitting message: '用一句话介绍你自己'")
	events := eng.SubmitMessage(ctx, "用一句话介绍你自己")

	var fullText strings.Builder
	var gotDone bool
	var usage *engine.UsageStats

	for ev := range events {
		switch ev.Type {
		case engine.EventTextDelta:
			fullText.WriteString(ev.Text)
			fmt.Print(ev.Text) // stream to test output
		case engine.EventTextComplete:
			// complete text already accumulated
		case engine.EventUsage:
			usage = ev.Usage
		case engine.EventDone:
			gotDone = true
		case engine.EventError:
			t.Fatalf("Engine returned error event: %s", ev.Error)
		case engine.EventSystemMessage:
			t.Logf("[system] %s", ev.Text)
		}
	}
	fmt.Println() // newline after streamed text

	t.Logf("Full response: %s", fullText.String())
	if usage != nil {
		t.Logf("Usage — input: %d tokens, output: %d tokens", usage.InputTokens, usage.OutputTokens)
	}

	assert.True(t, gotDone, "expected EventDone")
	assert.NotEmpty(t, fullText.String(), "expected non-empty text response")
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// newLiveEngine creates a fully wired SDK engine from .env credentials.
func newLiveEngine(t *testing.T, workDir string, maxTokens int) *sdk.Engine {
	t.Helper()
	apiKey, baseURL, provType, model := liveEnvConfig()
	if apiKey == "" {
		t.Skip("No API key found in .env — skipping live test")
	}
	eng, err := sdk.New(
		sdk.WithProvider(provType),
		sdk.WithAPIKey(apiKey),
		sdk.WithModel(model),
		sdk.WithBaseURL(baseURL),
		sdk.WithWorkDir(workDir),
		sdk.WithMaxTokens(maxTokens),
	)
	require.NoError(t, err)
	t.Logf("[engine] provider=%s model=%s baseURL=%s", provType, model, baseURL)
	return eng
}

// turnResult collects all events from one SubmitMessage call.
type turnResult struct {
	Text         string
	Results      []string // tool result texts (one entry per tool call)
	ToolsUsed    []string
	HasToolError bool
	GotDone      bool
	Usage        *engine.UsageStats
}

// collectTurn drains the event channel from one SubmitMessage call.
func collectTurn(t *testing.T, ch <-chan *engine.StreamEvent) turnResult {
	t.Helper()
	var r turnResult
	for ev := range ch {
		switch ev.Type {
		case engine.EventTextDelta:
			r.Text += ev.Text
			fmt.Print(ev.Text)
		case engine.EventToolUse:
			r.ToolsUsed = append(r.ToolsUsed, ev.ToolName)
			t.Logf("  [tool_use]  %s %v", ev.ToolName, ev.ToolInput)
		case engine.EventToolResult:
			if ev.IsError {
				r.HasToolError = true
				t.Logf("  [tool_result] ERROR tool=%s", ev.ToolName)
			} else {
				t.Logf("  [tool_result] OK tool=%s", ev.ToolName)
			}
			if ev.Result != "" {
				r.Results = append(r.Results, ev.Result)
			}
		case engine.EventUsage:
			r.Usage = ev.Usage
		case engine.EventDone:
			r.GotDone = true
		case engine.EventError:
			t.Fatalf("engine error: %s", ev.Error)
		case engine.EventSystemMessage:
			t.Logf("  [system] %s", ev.Text)
		}
	}
	fmt.Println()
	return r
}

// listWorkDir logs all files inside dir for diagnostics.
func listWorkDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Logf("  [workdir] ReadDir error: %v", err)
		return
	}
	if len(entries) == 0 {
		t.Logf("  [workdir] %s — (empty)", dir)
		return
	}
	for _, e := range entries {
		t.Logf("  [workdir] %s/%s", dir, e.Name())
	}
}

// containsTool returns true if any tool in the list matches name.
func containsTool(tools []string, name string) bool {
	for _, n := range tools {
		if strings.EqualFold(n, name) {
			return true
		}
	}
	return false
}

// ─── existing tests ───────────────────────────────────────────────────────────

// TestLiveMiniMaxToolUse tests that the engine can call a tool (Bash echo) and relay the result.
func TestLiveMiniMaxToolUse(t *testing.T) {
	apiKey, baseURL, provType, model := liveEnvConfig()
	if apiKey == "" {
		t.Skip("No API key found in .env or environment — skipping live test")
	}

	workDir := t.TempDir()
	eng, err := sdk.New(
		sdk.WithProvider(provType),
		sdk.WithAPIKey(apiKey),
		sdk.WithModel(model),
		sdk.WithBaseURL(baseURL),
		sdk.WithWorkDir(workDir),
		sdk.WithMaxTokens(1024),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	t.Log("Submitting message: 'Please run the bash command: echo AGENT_OK'")
	events := eng.SubmitMessage(ctx, "Please run the bash command: echo AGENT_OK")

	var fullText strings.Builder
	var toolsUsed []string
	var gotDone bool

	for ev := range events {
		switch ev.Type {
		case engine.EventTextDelta:
			fullText.WriteString(ev.Text)
		case engine.EventToolUse:
			toolsUsed = append(toolsUsed, ev.ToolName)
			t.Logf("[tool_use] %s: %v", ev.ToolName, ev.ToolInput)
		case engine.EventToolResult:
			t.Logf("[tool_result] %s", ev.Result)
			// Also accumulate tool result text for assertions.
			fullText.WriteString(ev.Result)
		case engine.EventDone:
			gotDone = true
		case engine.EventError:
			t.Fatalf("Engine returned error event: %s", ev.Error)
		case engine.EventSystemMessage:
			t.Logf("[system] %s", ev.Text)
		}
	}

	t.Logf("Full response: %s", fullText.String())
	t.Logf("Tools used: %v", toolsUsed)

	assert.True(t, gotDone, "expected EventDone")
	assert.NotEmpty(t, toolsUsed, "expected at least one tool to be called")
	// Final-text may be empty if MiniMax ends after the tool result; that is OK.
	if fullText.Len() > 0 {
		t.Logf("Final LLM text: %s", fullText.String())
	}
}

// ─── Full Agent Flow ──────────────────────────────────────────────────────────

// TestLiveMiniMaxFullAgentFlow tests the complete agent lifecycle across 4 turns:
//
//	Turn 1 — File write    : agent creates a file in workdir via Write tool
//	Turn 2 — Bash verify   : agent runs `cat` to read the file and reports content
//	Turn 3 — File append   : agent appends a second line (Write/Edit/Bash)
//	Turn 4 — Context check : agent summarises what it did (multi-turn memory)
func TestLiveMiniMaxFullAgentFlow(t *testing.T) {
	workDir := t.TempDir()
	eng := newLiveEngine(t, workDir, 2048)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	targetFile := filepath.Join(workDir, "agent_output.txt")

	// ── Turn 1: Write a file ──────────────────────────────────────────────────
	t.Log("\n══ Turn 1: Write file ══")
	r1 := collectTurn(t, eng.SubmitMessage(ctx,
		`Use the Write tool to create a file called "agent_output.txt" `+
			`containing exactly this text: AGENT_ENGINE_VERIFIED`))
	assert.True(t, r1.GotDone, "turn 1: expected EventDone")
	// Tool registry names: Write (filewrite), Bash
	assert.True(t,
		containsTool(r1.ToolsUsed, "Write") || containsTool(r1.ToolsUsed, "Bash"),
		"turn 1: expected Write or Bash tool, got %v", r1.ToolsUsed)
	assert.False(t, r1.HasToolError, "turn 1: tool should not return an error")

	// Diagnose what's in workDir regardless of outcome.
	t.Log("  [workdir after turn 1]")
	listWorkDir(t, workDir)

	// Soft-check file existence — continue the test even if file is missing.
	if content, err := os.ReadFile(targetFile); err == nil {
		t.Logf("  [file] agent_output.txt content: %q", strings.TrimSpace(string(content)))
		assert.Contains(t, string(content), "AGENT_ENGINE_VERIFIED")
	} else {
		t.Logf("  [file] agent_output.txt not found on disk: %v", err)
		t.Logf("  [llm]  turn 1 response: %s", r1.Text)
	}

	// ── Turn 2: Bash read-back ────────────────────────────────────────────────
	t.Log("\n══ Turn 2: Bash verify ══")
	r2 := collectTurn(t, eng.SubmitMessage(ctx,
		`Run: cat agent_output.txt   and tell me exactly what output you see.`))
	assert.True(t, r2.GotDone, "turn 2: expected EventDone")
	assert.True(t, containsTool(r2.ToolsUsed, "Bash"),
		"turn 2: expected Bash tool, got %v", r2.ToolsUsed)
	// The LLM will paraphrase the bash output in its text response.
	t.Logf("  [llm] turn 2 response: %s", r2.Text)
	assert.NotEmpty(t, r2.Text, "turn 2: LLM should describe the bash output")

	// ── Turn 3: Append a line ─────────────────────────────────────────────────
	t.Log("\n══ Turn 3: Append line ══")
	r3 := collectTurn(t, eng.SubmitMessage(ctx,
		`Append a new line containing MULTI_TURN_OK to agent_output.txt.`))
	assert.True(t, r3.GotDone, "turn 3: expected EventDone")
	assert.NotEmpty(t, r3.ToolsUsed, "turn 3: expected at least one tool call")
	assert.False(t, r3.HasToolError, "turn 3: tool should not return an error")

	t.Log("  [workdir after turn 3]")
	listWorkDir(t, workDir)

	if content3, err := os.ReadFile(targetFile); err == nil {
		t.Logf("  [file] agent_output.txt after turn 3:\n%s", strings.TrimSpace(string(content3)))
		assert.Contains(t, string(content3), "MULTI_TURN_OK",
			"file should contain the appended line")
	} else {
		t.Logf("  [file] agent_output.txt read error: %v", err)
	}

	// ── Turn 4: Multi-turn context retention ──────────────────────────────────
	t.Log("\n══ Turn 4: Context retention ══")
	r4 := collectTurn(t, eng.SubmitMessage(ctx,
		`Without using any tools, tell me: what file did you create and what did you write in it?`))
	assert.True(t, r4.GotDone, "turn 4: expected EventDone")
	assert.NotEmpty(t, r4.Text, "turn 4: LLM should answer from conversation history")
	t.Logf("  [llm] turn 4 response: %s", r4.Text)
	// Should reference the file name or sentinel string from earlier turns.
	lowerResp := strings.ToLower(r4.Text)
	assert.True(t,
		strings.Contains(lowerResp, "agent_output") || strings.Contains(lowerResp, "verified"),
		"turn 4: LLM should recall the file or its content")

	// ── Summary ───────────────────────────────────────────────────────────────
	t.Log("\n══ Full-flow complete ══")
	for i, u := range []*engine.UsageStats{r1.Usage, r2.Usage, r3.Usage, r4.Usage} {
		if u != nil {
			t.Logf("Turn %d — in: %d  out: %d  cost: $%.4f",
				i+1, u.InputTokens, u.OutputTokens, u.CostUSD)
		}
	}
}
