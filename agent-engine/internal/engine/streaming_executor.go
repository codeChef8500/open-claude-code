package engine

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// StreamingToolExecutor — executes tools with real-time progress streaming.
// Aligned with claude-code-main's streaming tool execution pattern where
// partial results are emitted as they become available.
// ────────────────────────────────────────────────────────────────────────────

// StreamingToolExecutor wraps ToolExecutor to emit progress events during
// parallel tool execution. Each tool's output blocks are streamed as they
// arrive, and a final aggregated result is emitted when all tools complete.
type StreamingToolExecutor struct {
	executor       *ToolExecutor
	maxConcurrency int
	eventSink      func(StreamEvent)
}

// NewStreamingToolExecutor creates a streaming executor.
// eventSink receives real-time events; pass nil for silent execution.
func NewStreamingToolExecutor(exec *ToolExecutor, maxConcurrency int, sink func(StreamEvent)) *StreamingToolExecutor {
	if sink == nil {
		sink = func(StreamEvent) {}
	}
	return &StreamingToolExecutor{
		executor:       exec,
		maxConcurrency: maxConcurrency,
		eventSink:      sink,
	}
}

// StreamingExecRequest extends ToolExecRequest with streaming metadata.
type StreamingExecRequest struct {
	*ToolExecRequest
	// Index is the position within the batch (for deterministic ordering).
	Index int
}

// StreamingExecResult extends ToolExecResult with timing and ordering info.
type StreamingExecResult struct {
	*ToolExecResult
	// Index matches StreamingExecRequest.Index.
	Index int
	// StartTime is when execution began.
	StartTime time.Time
	// EndTime is when execution completed.
	EndTime time.Time
}

// ExecuteStreaming runs all requests with concurrent streaming.
// Progress events are emitted via eventSink as each tool starts, produces
// output, and finishes. Results are returned in input order.
func (se *StreamingToolExecutor) ExecuteStreaming(ctx context.Context, reqs []*StreamingExecRequest) []*StreamingExecResult {
	if len(reqs) == 0 {
		return nil
	}

	results := make([]*StreamingExecResult, len(reqs))
	var wg sync.WaitGroup

	var sem chan struct{}
	if se.maxConcurrency > 0 {
		sem = make(chan struct{}, se.maxConcurrency)
	}

	for _, req := range reqs {
		wg.Add(1)
		go func(r *StreamingExecRequest) {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}

			startTime := time.Now()

			// Emit tool start event.
			se.eventSink(StreamEvent{
				Type:     EventToolUse,
				ToolName: r.Tool.Name(),
				ToolID:   r.ToolUseID,
			})

			// Execute the tool.
			execResult := se.executor.Execute(ctx, r.ToolExecRequest)

			endTime := time.Now()

			// Emit tool result event.
			resultText := blocksToString(execResult.Blocks)
			se.eventSink(StreamEvent{
				Type:     EventToolResult,
				ToolName: r.Tool.Name(),
				ToolID:   r.ToolUseID,
				Result:   resultText,
				IsError:  execResult.IsError,
			})

			results[r.Index] = &StreamingExecResult{
				ToolExecResult: execResult,
				Index:          r.Index,
				StartTime:      startTime,
				EndTime:        endTime,
			}
		}(req)
	}

	wg.Wait()
	return results
}

// ExecuteWithProgress runs a single tool and emits periodic progress events.
// progressInterval controls how often progress events are emitted during
// long-running tools.
func (se *StreamingToolExecutor) ExecuteWithProgress(ctx context.Context, req *ToolExecRequest, progressInterval time.Duration) *ToolExecResult {
	if progressInterval <= 0 {
		progressInterval = 2 * time.Second
	}

	// Start progress ticker.
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(progressInterval)
		defer ticker.Stop()
		elapsed := time.Duration(0)
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				elapsed += progressInterval
				se.eventSink(StreamEvent{
					Type:     EventToolProgress,
					ToolName: req.Tool.Name(),
					ToolID:   req.ToolUseID,
					Progress: &ProgressData{
						Content: "running (" + elapsed.String() + ")",
					},
				})
			}
		}
	}()

	result := se.executor.Execute(ctx, req)
	close(done)

	return result
}

// (ProgressData is defined in types.go)

// blocksToString concatenates text blocks into a single string.
func blocksToString(blocks []*ContentBlock) string {
	if len(blocks) == 0 {
		return ""
	}
	if len(blocks) == 1 {
		return blocks[0].Text
	}
	total := 0
	for _, b := range blocks {
		total += len(b.Text)
	}
	buf := make([]byte, 0, total+len(blocks))
	for i, b := range blocks {
		if i > 0 {
			buf = append(buf, '\n')
		}
		buf = append(buf, b.Text...)
	}
	return string(buf)
}

// ────────────────────────────────────────────────────────────────────────────
// ToolUseSummary — generates compact summaries for tool use display.
// Aligned with claude-code-main's generateToolUseSummary.
// ────────────────────────────────────────────────────────────────────────────

// ToolUseSummaryExt holds extended summary information about a completed tool use.
type ToolUseSummaryExt struct {
	ToolName string          `json:"tool_name"`
	ToolID   string          `json:"tool_id"`
	Duration time.Duration   `json:"duration"`
	IsError  bool            `json:"is_error"`
	Summary  string          `json:"summary"`
	Input    json.RawMessage `json:"input,omitempty"`
}

// GenerateToolUseSummary creates a compact summary of a tool execution result.
func GenerateToolUseSummary(result *ToolExecResult, input json.RawMessage) *ToolUseSummaryExt {
	summary := &ToolUseSummaryExt{
		ToolName: result.ToolName,
		ToolID:   result.ToolUseID,
		Duration: result.Duration,
		IsError:  result.IsError,
		Input:    input,
	}

	// Generate a compact summary string.
	if result.IsError {
		summary.Summary = summarizeError(result)
	} else {
		summary.Summary = summarizeSuccess(result)
	}

	return summary
}

func summarizeError(result *ToolExecResult) string {
	for _, b := range result.Blocks {
		if b.IsError && b.Text != "" {
			text := b.Text
			if len(text) > 200 {
				text = text[:200] + "..."
			}
			return text
		}
	}
	return "tool execution failed"
}

func summarizeSuccess(result *ToolExecResult) string {
	totalChars := 0
	for _, b := range result.Blocks {
		totalChars += len(b.Text)
	}

	switch {
	case totalChars == 0:
		return "(no output)"
	case totalChars <= 100:
		return blocksToString(result.Blocks)
	default:
		text := blocksToString(result.Blocks)
		lines := 0
		for _, c := range text {
			if c == '\n' {
				lines++
			}
		}
		return formatSummaryStats(totalChars, lines)
	}
}

func formatSummaryStats(chars, lines int) string {
	if lines > 1 {
		return formatCompactSize(chars) + " (" + formatInt(lines) + " lines)"
	}
	return formatCompactSize(chars)
}

func formatCompactSize(chars int) string {
	switch {
	case chars < 1024:
		return formatInt(chars) + " chars"
	case chars < 1024*1024:
		return formatFloat(float64(chars)/1024) + " KB"
	default:
		return formatFloat(float64(chars)/(1024*1024)) + " MB"
	}
}

func formatInt(n int) string {
	if n < 0 {
		return "-" + formatInt(-n)
	}
	s := ""
	for n > 0 {
		if s != "" && len(s)%4 == 3 {
			s = "," + s
		}
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if s == "" {
		return "0"
	}
	return s
}

func formatFloat(f float64) string {
	// Simple formatting: one decimal place.
	whole := int(f)
	frac := int((f - float64(whole)) * 10)
	if frac == 0 {
		return formatInt(whole)
	}
	return formatInt(whole) + "." + string(rune('0'+frac))
}

// EmitToolUseSummary sends a tool use summary event.
func EmitToolUseSummary(sink func(StreamEvent), result *ToolExecResult, input json.RawMessage) {
	if sink == nil {
		return
	}
	summary := GenerateToolUseSummary(result, input)
	sink(StreamEvent{
		Type:     EventToolUseSummary,
		ToolName: summary.ToolName,
		ToolID:   summary.ToolID,
		ToolUseSummary: &ToolUseSummaryData{
			ToolUseID:  summary.ToolID,
			ToolName:   summary.ToolName,
			Summary:    summary.Summary,
			DurationMs: int(summary.Duration.Milliseconds()),
		},
	})

	slog.Debug("tool use summary",
		slog.String("tool", summary.ToolName),
		slog.Duration("duration", summary.Duration),
		slog.Bool("error", summary.IsError),
		slog.String("summary", summary.Summary))
}
