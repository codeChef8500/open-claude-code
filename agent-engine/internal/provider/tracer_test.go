package provider

import (
	"context"
	"testing"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// tracerMockCaller returns events with usage info.
type tracerMockCaller struct{}

func (t *tracerMockCaller) Name() string { return "tracer-mock" }

func (t *tracerMockCaller) CallModel(_ context.Context, params engine.CallParams) (<-chan *engine.StreamEvent, error) {
	ch := make(chan *engine.StreamEvent, 3)
	ch <- &engine.StreamEvent{Type: engine.EventTextDelta, Text: "hello"}
	ch <- &engine.StreamEvent{Type: engine.EventUsage, Usage: &engine.UsageStats{
		InputTokens:  100,
		OutputTokens: 50,
	}}
	close(ch)
	return ch, nil
}

func TestRequestTracer_CallModel(t *testing.T) {
	inner := &tracerMockCaller{}
	tracer := NewRequestTracer(inner, nil, 100)

	ch, err := tracer.CallModel(context.Background(), engine.CallParams{Model: "test-model"})
	if err != nil {
		t.Fatalf("CallModel: %v", err)
	}

	var text string
	for ev := range ch {
		if ev.Type == engine.EventTextDelta {
			text += ev.Text
		}
	}
	if text != "hello" {
		t.Errorf("expected 'hello', got %q", text)
	}

	// Allow goroutine to finish recording.
	time.Sleep(10 * time.Millisecond)

	traces := tracer.Traces()
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	tr := traces[0]
	if tr.Model != "test-model" {
		t.Errorf("model: expected 'test-model', got %q", tr.Model)
	}
	if !tr.Success {
		t.Error("expected success")
	}
	if tr.InputTokens != 100 {
		t.Errorf("input tokens: expected 100, got %d", tr.InputTokens)
	}
	if tr.OutputTokens != 50 {
		t.Errorf("output tokens: expected 50, got %d", tr.OutputTokens)
	}
	if tr.Duration < 0 {
		t.Error("expected non-negative duration")
	}
}

func TestRequestTracer_Recent(t *testing.T) {
	inner := &tracerMockCaller{}
	tracer := NewRequestTracer(inner, nil, 100)

	for i := 0; i < 5; i++ {
		ch, _ := tracer.CallModel(context.Background(), engine.CallParams{Model: "m"})
		for range ch {
		}
	}
	time.Sleep(20 * time.Millisecond)

	recent := tracer.Recent(3)
	if len(recent) != 3 {
		t.Errorf("expected 3 recent, got %d", len(recent))
	}

	all := tracer.Recent(0)
	if len(all) != 5 {
		t.Errorf("expected 5 for n=0, got %d", len(all))
	}
}

func TestRequestTracer_Stats(t *testing.T) {
	inner := &tracerMockCaller{}
	tracer := NewRequestTracer(inner, nil, 100)

	for i := 0; i < 3; i++ {
		ch, _ := tracer.CallModel(context.Background(), engine.CallParams{Model: "m"})
		for range ch {
		}
	}
	time.Sleep(20 * time.Millisecond)

	stats := tracer.Stats()
	if stats.TotalRequests != 3 {
		t.Errorf("total: expected 3, got %d", stats.TotalRequests)
	}
	if stats.SuccessCount != 3 {
		t.Errorf("successes: expected 3, got %d", stats.SuccessCount)
	}
	if stats.ErrorCount != 0 {
		t.Errorf("errors: expected 0, got %d", stats.ErrorCount)
	}
	if stats.TotalInputTok != 300 {
		t.Errorf("input tokens: expected 300, got %d", stats.TotalInputTok)
	}
	if stats.TotalOutputTok != 150 {
		t.Errorf("output tokens: expected 150, got %d", stats.TotalOutputTok)
	}
}

func TestRequestTracer_Clear(t *testing.T) {
	inner := &tracerMockCaller{}
	tracer := NewRequestTracer(inner, nil, 100)

	ch, _ := tracer.CallModel(context.Background(), engine.CallParams{})
	for range ch {
	}
	time.Sleep(10 * time.Millisecond)

	tracer.Clear()
	if len(tracer.Traces()) != 0 {
		t.Errorf("expected 0 after clear, got %d", len(tracer.Traces()))
	}
}

func TestRequestTracer_MaxSize(t *testing.T) {
	inner := &tracerMockCaller{}
	tracer := NewRequestTracer(inner, nil, 3)

	for i := 0; i < 10; i++ {
		ch, _ := tracer.CallModel(context.Background(), engine.CallParams{})
		for range ch {
		}
	}
	time.Sleep(20 * time.Millisecond)

	if len(tracer.Traces()) > 3 {
		t.Errorf("expected max 3 traces, got %d", len(tracer.Traces()))
	}
}

func TestRequestTracer_DefaultMaxSize(t *testing.T) {
	tracer := NewRequestTracer(&tracerMockCaller{}, nil, 0)
	if tracer.maxSize != 200 {
		t.Errorf("expected default 200, got %d", tracer.maxSize)
	}
}
