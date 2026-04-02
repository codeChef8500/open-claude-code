package hooks

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRegistry_RegisterAndRunSync(t *testing.T) {
	reg := NewRegistry()
	var called bool
	reg.Register(EventPreToolUse, "test-hook", func(_ context.Context, input *HookInput) (*HookJSONOutput, error) {
		called = true
		return nil, nil
	})

	if !reg.HasHandlers(EventPreToolUse) {
		t.Error("expected handlers for PreToolUse")
	}

	resp := reg.RunSync(context.Background(), EventPreToolUse, &HookInput{})
	if !called {
		t.Error("handler was not called")
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
}

func TestRegistry_BlockDecision(t *testing.T) {
	reg := NewRegistry()
	reg.Register(EventPreToolUse, "blocker", func(_ context.Context, _ *HookInput) (*HookJSONOutput, error) {
		return &HookJSONOutput{Decision: "block", FailureReason: "blocked by test"}, nil
	})
	reg.Register(EventPreToolUse, "after-blocker", func(_ context.Context, _ *HookInput) (*HookJSONOutput, error) {
		t.Error("should not be called after block")
		return nil, nil
	})

	resp := reg.RunSync(context.Background(), EventPreToolUse, &HookInput{})
	if resp.Decision != "block" {
		t.Errorf("expected block, got %q", resp.Decision)
	}
	if resp.FailureReason != "blocked by test" {
		t.Errorf("expected failure reason, got %q", resp.FailureReason)
	}
}

func TestRegistry_RunAsync(t *testing.T) {
	reg := NewRegistry()
	var called atomic.Int32
	reg.RegisterAsync(EventPostToolUse, "async-hook", func(_ context.Context, _ *HookInput) (*HookJSONOutput, error) {
		called.Add(1)
		return nil, nil
	})

	reg.RunAsync(EventPostToolUse, &HookInput{})
	time.Sleep(50 * time.Millisecond)
	if called.Load() != 1 {
		t.Errorf("expected 1 async call, got %d", called.Load())
	}
}

func TestRegistry_Unregister(t *testing.T) {
	reg := NewRegistry()
	reg.Register(EventPreToolUse, "removeme", func(_ context.Context, _ *HookInput) (*HookJSONOutput, error) {
		t.Error("should not be called after unregister")
		return nil, nil
	})
	reg.Unregister(EventPreToolUse, "removeme")

	if reg.HasHandlers(EventPreToolUse) {
		t.Error("expected no handlers after unregister")
	}
}

func TestRegistry_NoHandlers(t *testing.T) {
	reg := NewRegistry()
	resp := reg.RunSync(context.Background(), EventPreToolUse, &HookInput{})
	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
	if resp.Decision != "" {
		t.Errorf("expected empty decision, got %q", resp.Decision)
	}
}
