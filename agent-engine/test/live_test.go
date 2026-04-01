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
	"github.com/wall-ai/agent-engine/internal/provider"
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
		if os.Getenv(key) == "" { // don't overwrite already-set vars
			_ = os.Setenv(key, val)
		}
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
// Falls back to common variable name aliases.
func liveEnvConfig() (apiKey, baseURL, provType, model string) {
	// Load .env if present.
	envFile := filepath.Join(projectRoot(), ".env")
	_ = loadDotEnv(envFile)

	// Read with AGENT_ENGINE_ prefix first, then bare names.
	get := func(keys ...string) string {
		for _, k := range keys {
			if v := os.Getenv(k); v != "" {
				return v
			}
		}
		return ""
	}

	apiKey   = get("AGENT_ENGINE_API_KEY", "MINIMAX_API_KEY", "OPENAI_API_KEY")
	baseURL  = get("AGENT_ENGINE_BASE_URL", "MINIMAX_BASE_URL", "OPENAI_BASE_URL")
	provType = get("AGENT_ENGINE_PROVIDER", "LLM_PROVIDER")
	model    = get("AGENT_ENGINE_MODEL", "LLM_MODEL")

	if provType == "" {
		provType = "openai" // MiniMax is OpenAI-compatible
	}
	if model == "" {
		model = "MiniMax-M2.5"
	}
	return
}

// TestLiveMiniMaxSimpleChat sends one message and verifies the streaming text response.
func TestLiveMiniMaxSimpleChat(t *testing.T) {
	apiKey, baseURL, provType, model := liveEnvConfig()
	if apiKey == "" {
		t.Skip("No API key found in .env or environment — skipping live test")
	}

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
	events := eng.SubmitMessage(ctx, engine.QueryParams{Text: "用一句话介绍你自己"})

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
	events := eng.SubmitMessage(ctx, engine.QueryParams{
		Text: "Please run the bash command: echo AGENT_OK",
	})

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
	assert.NotEmpty(t, fullText.String(), "expected non-empty final text")
}
