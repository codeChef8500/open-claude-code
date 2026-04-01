package test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// mockProvider is a minimal ModelCaller that returns a canned response.
type mockProvider struct {
	response string
	delay    time.Duration
}

func (m *mockProvider) Name() string { return "mock" }

func (m *mockProvider) CallModel(_ context.Context, params engine.CallParams) (<-chan *engine.StreamEvent, error) {
	ch := make(chan *engine.StreamEvent, 8)
	go func() {
		defer close(ch)
		if m.delay > 0 {
			time.Sleep(m.delay)
		}
		ch <- &engine.StreamEvent{Type: engine.EventTextDelta, Text: m.response}
		ch <- &engine.StreamEvent{Type: engine.EventUsage, Usage: &engine.UsageStats{
			InputTokens:  100,
			OutputTokens: 50,
		}}
		ch <- &engine.StreamEvent{Type: engine.EventDone}
	}()
	return ch, nil
}

func TestEngineSubmitMessage(t *testing.T) {
	prov := &mockProvider{response: "Hello, world!"}

	e, err := engine.New(engine.EngineConfig{
		Provider:  "anthropic",
		Model:     "claude-sonnet-4-5",
		MaxTokens: 8192,
		WorkDir:   t.TempDir(),
		SessionID: "test-session-1",
	}, prov, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := e.SubmitMessage(ctx, engine.QueryParams{Text: "Hi"})

	var textParts []string
	var gotDone bool
	for ev := range ch {
		switch ev.Type {
		case engine.EventTextDelta:
			textParts = append(textParts, ev.Text)
		case engine.EventDone:
			gotDone = true
		case engine.EventError:
			t.Fatalf("unexpected error event: %s", ev.Error)
		}
	}

	assert.True(t, gotDone, "expected EventDone")
	assert.Equal(t, "Hello, world!", strings.Join(textParts, ""))
}

func TestEngineContextCancellation(t *testing.T) {
	prov := &mockProvider{response: "slow response", delay: 500 * time.Millisecond}

	e, err := engine.New(engine.EngineConfig{
		Provider:  "anthropic",
		Model:     "claude-sonnet-4-5",
		MaxTokens: 8192,
		WorkDir:   t.TempDir(),
	}, prov, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ch := e.SubmitMessage(ctx, engine.QueryParams{Text: "Hi"})

	// Drain channel — should complete quickly without hanging.
	timeout := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return // channel closed, test passes
			}
		case <-timeout:
			t.Fatal("engine did not respect context cancellation")
		}
	}
}

func TestEngineAutoCompactTrigger(t *testing.T) {
	callCount := 0
	prov := &mockProvider{response: "compact me"}

	// Use very low MaxTokens so compact triggers immediately.
	e, err := engine.New(engine.EngineConfig{
		Provider:  "anthropic",
		Model:     "claude-sonnet-4-5",
		MaxTokens: 10, // tiny — triggers compact on any content
		WorkDir:   t.TempDir(),
	}, prov, nil)
	require.NoError(t, err)
	_ = callCount

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := e.SubmitMessage(ctx, engine.QueryParams{Text: "Hello"})
	for range ch {
	}
	// If we reach here without panic, compact logic is safe.
}
