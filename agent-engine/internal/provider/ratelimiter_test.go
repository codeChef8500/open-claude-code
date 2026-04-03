package provider

import (
	"context"
	"testing"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// mockCaller is a minimal ModelCaller for testing.
type mockCaller struct {
	callCount int
}

func (m *mockCaller) Name() string { return "mock" }

func (m *mockCaller) CallModel(_ context.Context, _ engine.CallParams) (<-chan *engine.StreamEvent, error) {
	m.callCount++
	ch := make(chan *engine.StreamEvent, 1)
	ch <- &engine.StreamEvent{Type: engine.EventTextDelta, Text: "ok"}
	close(ch)
	return ch, nil
}

func TestRateLimiter_CallModel(t *testing.T) {
	inner := &mockCaller{}
	rl := NewRateLimiter(inner, DefaultRateLimiterConfig())

	ch, err := rl.CallModel(context.Background(), engine.CallParams{})
	if err != nil {
		t.Fatalf("CallModel: %v", err)
	}
	var text string
	for ev := range ch {
		text += ev.Text
	}
	if text != "ok" {
		t.Errorf("expected 'ok', got %q", text)
	}
	if inner.callCount != 1 {
		t.Errorf("expected 1 call, got %d", inner.callCount)
	}
}

func TestRateLimiter_Available(t *testing.T) {
	rl := NewRateLimiter(&mockCaller{}, RateLimiterConfig{
		RequestsPerMinute: 60,
		BurstSize:         3,
	})

	avail := rl.Available()
	if avail != 3 {
		t.Errorf("expected 3 available tokens, got %d", avail)
	}
}

func TestRateLimiter_ConsumesTokens(t *testing.T) {
	rl := NewRateLimiter(&mockCaller{}, RateLimiterConfig{
		RequestsPerMinute: 60,
		BurstSize:         2,
	})

	// Use both burst tokens.
	_, _ = rl.CallModel(context.Background(), engine.CallParams{})
	_, _ = rl.CallModel(context.Background(), engine.CallParams{})

	avail := rl.Available()
	if avail > 1 {
		t.Errorf("expected 0-1 tokens after 2 calls with burst=2, got %d", avail)
	}
}

func TestRateLimiter_CancelledContext(t *testing.T) {
	rl := NewRateLimiter(&mockCaller{}, RateLimiterConfig{
		RequestsPerMinute: 1,
		BurstSize:         1,
	})

	// Exhaust the single token.
	_, _ = rl.CallModel(context.Background(), engine.CallParams{})

	// Now a cancelled context should fail fast.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := rl.CallModel(ctx, engine.CallParams{})
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestRateLimiter_DefaultConfig(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	if cfg.RequestsPerMinute != 50 {
		t.Errorf("expected 50 rpm, got %d", cfg.RequestsPerMinute)
	}
	if cfg.BurstSize != 5 {
		t.Errorf("expected burst 5, got %d", cfg.BurstSize)
	}
}

func TestRateLimiter_InvalidConfig(t *testing.T) {
	rl := NewRateLimiter(&mockCaller{}, RateLimiterConfig{
		RequestsPerMinute: -1,
		BurstSize:         0,
	})
	// Should default to sensible values.
	if rl.maxTokens != 5 {
		t.Errorf("expected default burst 5, got %d", rl.maxTokens)
	}
}
