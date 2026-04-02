package provider

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// RequestTrace records metadata about a single provider API call.
type RequestTrace struct {
	ID           uint64        `json:"id"`
	Model        string        `json:"model"`
	StartedAt    time.Time     `json:"started_at"`
	Duration     time.Duration `json:"duration"`
	InputTokens  int           `json:"input_tokens"`
	OutputTokens int           `json:"output_tokens"`
	Error        string        `json:"error,omitempty"`
	Success      bool          `json:"success"`
}

// TracerStats holds aggregate request statistics.
type TracerStats struct {
	TotalRequests   int           `json:"total_requests"`
	SuccessCount    int           `json:"success_count"`
	ErrorCount      int           `json:"error_count"`
	TotalDuration   time.Duration `json:"total_duration"`
	AvgDuration     time.Duration `json:"avg_duration"`
	TotalInputTok   int           `json:"total_input_tokens"`
	TotalOutputTok  int           `json:"total_output_tokens"`
}

// RequestTracer wraps a ModelCaller and records traces for every API call.
type RequestTracer struct {
	inner   engine.ModelCaller
	logger  *slog.Logger

	mu      sync.RWMutex
	traces  []RequestTrace
	maxSize int
	nextID  atomic.Uint64
}

// NewRequestTracer creates a tracer that records up to maxSize recent traces.
func NewRequestTracer(inner engine.ModelCaller, logger *slog.Logger, maxSize int) *RequestTracer {
	if logger == nil {
		logger = slog.Default()
	}
	if maxSize <= 0 {
		maxSize = 200
	}
	return &RequestTracer{
		inner:   inner,
		logger:  logger,
		traces:  make([]RequestTrace, 0, maxSize),
		maxSize: maxSize,
	}
}

// CallModel forwards the call to the inner provider, recording a trace.
func (rt *RequestTracer) CallModel(ctx context.Context, params engine.CallParams) (<-chan *engine.StreamEvent, error) {
	id := rt.nextID.Add(1)
	start := time.Now()

	ch, err := rt.inner.CallModel(ctx, params)
	if err != nil {
		rt.record(RequestTrace{
			ID:        id,
			Model:     params.Model,
			StartedAt: start,
			Duration:  time.Since(start),
			Error:     err.Error(),
			Success:   false,
		})
		return nil, err
	}

	// Wrap channel to capture usage and completion.
	out := make(chan *engine.StreamEvent, cap(ch))
	go func() {
		defer close(out)
		var (
			inputTok  int
			outputTok int
			gotError  string
		)
		for ev := range ch {
			out <- ev
			if ev == nil {
				continue
			}
			switch ev.Type {
			case engine.EventUsage:
				if ev.Usage != nil {
					inputTok = ev.Usage.InputTokens
					outputTok = ev.Usage.OutputTokens
				}
			case engine.EventError:
				gotError = ev.Error
			}
		}
		rt.record(RequestTrace{
			ID:           id,
			Model:        params.Model,
			StartedAt:    start,
			Duration:     time.Since(start),
			InputTokens:  inputTok,
			OutputTokens: outputTok,
			Error:        gotError,
			Success:      gotError == "",
		})
	}()

	return out, nil
}

func (rt *RequestTracer) record(trace RequestTrace) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.traces = append(rt.traces, trace)
	if len(rt.traces) > rt.maxSize {
		rt.traces = rt.traces[len(rt.traces)-rt.maxSize:]
	}
	rt.logger.Debug("provider: request traced",
		slog.Uint64("trace_id", trace.ID),
		slog.String("model", trace.Model),
		slog.Duration("duration", trace.Duration),
		slog.Bool("success", trace.Success))
}

// Traces returns a copy of all recorded traces.
func (rt *RequestTracer) Traces() []RequestTrace {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	out := make([]RequestTrace, len(rt.traces))
	copy(out, rt.traces)
	return out
}

// Recent returns the N most recent traces.
func (rt *RequestTracer) Recent(n int) []RequestTrace {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	if n <= 0 || n > len(rt.traces) {
		n = len(rt.traces)
	}
	start := len(rt.traces) - n
	out := make([]RequestTrace, n)
	copy(out, rt.traces[start:])
	return out
}

// Stats returns aggregate statistics.
func (rt *RequestTracer) Stats() TracerStats {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	var stats TracerStats
	stats.TotalRequests = len(rt.traces)
	for _, t := range rt.traces {
		if t.Success {
			stats.SuccessCount++
		} else {
			stats.ErrorCount++
		}
		stats.TotalDuration += t.Duration
		stats.TotalInputTok += t.InputTokens
		stats.TotalOutputTok += t.OutputTokens
	}
	if stats.TotalRequests > 0 {
		stats.AvgDuration = stats.TotalDuration / time.Duration(stats.TotalRequests)
	}
	return stats
}

// Clear removes all recorded traces.
func (rt *RequestTracer) Clear() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.traces = rt.traces[:0]
}
