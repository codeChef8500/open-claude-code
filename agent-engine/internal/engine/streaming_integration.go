package engine

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
)

// ────────────────────────────────────────────────────────────────────────────
// Streaming tool execution integration — bridges the StreamingToolExecutor
// into the query loop's streaming receive loop.
// Aligned with claude-code-main query.ts:826-862 (addTool during streaming)
// and query.ts:1380-1408 (getRemainingResults after streaming).
// ────────────────────────────────────────────────────────────────────────────

// StreamingToolCollector manages tools that arrive during streaming and
// dispatches them to the StreamingToolExecutor for concurrent execution.
// It collects completed results as they become available.
type StreamingToolCollector struct {
	mu       sync.Mutex
	executor *StreamingToolExecutor
	ctx      context.Context

	// pending tracks tools that have been submitted but not yet collected.
	pending []*StreamingExecRequest
	// completed collects results as they finish.
	completed []*StreamingExecResult
	// nextIndex is the next request index.
	nextIndex int
	// toolRegistry maps tool names to Tool implementations.
	toolRegistry map[string]Tool
	// started is true after the first tool is added.
	started bool
	// allSubmitted is true when streaming is done and no more tools will arrive.
	allSubmitted bool
}

// NewStreamingToolCollector creates a collector that dispatches tools during
// streaming.  toolRegistry maps tool names to implementations.
func NewStreamingToolCollector(
	ctx context.Context,
	executor *StreamingToolExecutor,
	tools []Tool,
) *StreamingToolCollector {
	registry := make(map[string]Tool, len(tools))
	for _, t := range tools {
		registry[t.Name()] = t
	}
	return &StreamingToolCollector{
		executor:     executor,
		ctx:          ctx,
		toolRegistry: registry,
	}
}

// AddTool submits a tool_use block for execution as soon as it is received
// during streaming.  The tool begins executing immediately in the background.
// Aligned with claude-code-main StreamingToolExecutor.addTool.
func (c *StreamingToolCollector) AddTool(block *ContentBlock, assistantMsg *Message) {
	if block == nil || block.Type != ContentTypeToolUse {
		return
	}

	tool, ok := c.toolRegistry[block.ToolName]
	if !ok {
		slog.Warn("streaming_integration: unknown tool during streaming",
			slog.String("tool", block.ToolName),
			slog.String("tool_use_id", block.ToolUseID))
		return
	}

	c.mu.Lock()
	idx := c.nextIndex
	c.nextIndex++
	c.started = true

	// Marshal interface{} input to json.RawMessage for ToolExecRequest.
	var rawInput json.RawMessage
	if block.Input != nil {
		if rm, ok := block.Input.(json.RawMessage); ok {
			rawInput = rm
		} else if b, err := json.Marshal(block.Input); err == nil {
			rawInput = b
		}
	}

	req := &StreamingExecRequest{
		ToolExecRequest: &ToolExecRequest{
			Tool:      tool,
			ToolUseID: block.ToolUseID,
			Input:     rawInput,
		},
		Index: idx,
	}
	c.pending = append(c.pending, req)
	c.mu.Unlock()

	// Execute asynchronously — result is collected via GetCompletedResults.
	go func() {
		results := c.executor.ExecuteStreaming(c.ctx, []*StreamingExecRequest{req})
		c.mu.Lock()
		defer c.mu.Unlock()
		c.completed = append(c.completed, results...)
	}()
}

// GetCompletedResults returns any results that have completed since the last
// call.  Non-blocking: returns nil if nothing is ready.
// Aligned with claude-code-main StreamingToolExecutor.getCompletedResults.
func (c *StreamingToolCollector) GetCompletedResults() []*StreamingExecResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.completed) == 0 {
		return nil
	}
	results := c.completed
	c.completed = nil
	return results
}

// GetRemainingResults blocks until all pending tools complete and returns
// their results.  Called after streaming finishes.
// Aligned with claude-code-main StreamingToolExecutor.getRemainingResults.
func (c *StreamingToolCollector) GetRemainingResults() []*StreamingExecResult {
	c.mu.Lock()
	c.allSubmitted = true
	pending := make([]*StreamingExecRequest, len(c.pending))
	copy(pending, c.pending)
	c.mu.Unlock()

	// Wait for all to complete by executing any that haven't started.
	// The ones already running will finish on their own.
	// We just need to drain the completed list.
	for {
		c.mu.Lock()
		completedCount := len(c.completed)
		pendingCount := len(c.pending)
		c.mu.Unlock()

		if completedCount >= pendingCount {
			break
		}

		// Brief busy-wait (the goroutines will finish quickly).
		// In production, this would use a condition variable.
		select {
		case <-c.ctx.Done():
			return c.GetCompletedResults()
		default:
		}
	}

	return c.GetCompletedResults()
}

// HasPendingTools reports whether any tools have been submitted.
func (c *StreamingToolCollector) HasPendingTools() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.started
}

// Discard cancels all pending tools (used during fallback).
func (c *StreamingToolCollector) Discard() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending = nil
	c.completed = nil
	c.started = false
}

// ── Integration with query loop ──────────────────────────────────────────

// ShouldUseStreamingToolExec returns true if streaming tool execution should
// be enabled for this iteration.
func ShouldUseStreamingToolExec(gates QueryGates, toolCount int) bool {
	return gates.StreamingToolExecution && toolCount > 0
}

// BuildToolResultsFromStreaming converts StreamingExecResults into a tool
// result message that can be appended to the conversation.
func BuildToolResultsFromStreaming(results []*StreamingExecResult) *Message {
	var blocks []*ContentBlock

	for _, r := range results {
		if r == nil || r.ToolExecResult == nil {
			continue
		}
		block := &ContentBlock{
			Type:      ContentTypeToolResult,
			ToolUseID: r.ToolExecResult.ToolUseID,
			IsError:   r.ToolExecResult.IsError,
			Content:   r.ToolExecResult.Blocks,
		}
		blocks = append(blocks, block)
	}

	return &Message{
		Role:    RoleUser,
		Content: blocks,
	}
}
