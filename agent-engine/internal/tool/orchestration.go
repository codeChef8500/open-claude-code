package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/hooks"
)

// CallRequest is a single pending tool invocation.
type CallRequest struct {
	ToolUseID string
	ToolName  string
	Input     json.RawMessage
}

// CallResult holds the outcome of one tool invocation.
type CallResult struct {
	ToolUseID string
	Blocks    []*engine.ContentBlock
	IsError   bool
	// ContextModifier, if non-nil, should be applied to the UseContext after
	// this result is consumed (for sequential tools only; concurrent tools
	// queue modifiers and apply them after the batch completes).
	ContextModifier func(uctx *engine.UseContext) *engine.UseContext
}

// RunToolCallsOptions configures a RunToolCalls invocation.
type RunToolCallsOptions struct {
	// HookExecutor runs pre/post tool-use hooks. May be nil.
	HookExecutor *hooks.Executor
	// OnProgress is called with incremental progress updates.
	OnProgress func(CallResult)
}

// RunToolCalls executes a batch of tool calls. Concurrency-safe tools are
// executed in parallel via goroutines; others run sequentially.
// Aligned with claude-code-main's runTools (toolOrchestration.ts).
func RunToolCalls(
	ctx context.Context,
	registry *Registry,
	calls []CallRequest,
	uctx *UseContext,
) ([]CallResult, error) {
	return RunToolCallsWithOpts(ctx, registry, calls, uctx, RunToolCallsOptions{})
}

// RunToolCallsWithOpts executes tool calls with additional options.
func RunToolCallsWithOpts(
	ctx context.Context,
	registry *Registry,
	calls []CallRequest,
	uctx *UseContext,
	opts RunToolCallsOptions,
) ([]CallResult, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	// Partition into batches using engine.PartitionToolCalls.
	allTools := registry.All()
	toolSlice := make([]engine.Tool, len(allTools))
	for i, t := range allTools {
		toolSlice[i] = t
	}

	tcBlocks := make([]engine.ToolCallBlock, len(calls))
	for i, c := range calls {
		tcBlocks[i] = engine.ToolCallBlock{ID: c.ToolUseID, Name: c.ToolName, Input: c.Input}
	}
	batches := engine.PartitionToolCalls(tcBlocks, toolSlice)

	var allResults []CallResult
	currentUctx := uctx

	for _, batch := range batches {
		batchCalls := make([]CallRequest, len(batch.Calls))
		for i, tc := range batch.Calls {
			batchCalls[i] = CallRequest{ToolUseID: tc.ID, ToolName: tc.Name, Input: tc.Input}
		}

		if batch.IsConcurrencySafe {
			// Run concurrent batch.
			results := runConcurrentBatch(ctx, registry, batchCalls, currentUctx, opts)
			// Apply queued context modifiers after batch completes.
			for _, r := range results {
				if r.ContextModifier != nil {
					modified := r.ContextModifier(currentUctx)
					if modified != nil {
						currentUctx = modified
					}
				}
			}
			allResults = append(allResults, results...)
		} else {
			// Run sequential batch (one at a time).
			for _, c := range batchCalls {
				res := invokeWithHooks(ctx, registry, c, currentUctx, opts)
				allResults = append(allResults, res)
				// Apply context modifier immediately for sequential tools.
				if res.ContextModifier != nil {
					modified := res.ContextModifier(currentUctx)
					if modified != nil {
						currentUctx = modified
					}
				}
			}
		}
	}

	return allResults, nil
}

// runConcurrentBatch runs a batch of concurrency-safe tool calls in parallel.
func runConcurrentBatch(
	ctx context.Context,
	registry *Registry,
	calls []CallRequest,
	uctx *UseContext,
	opts RunToolCallsOptions,
) []CallResult {
	results := make([]CallResult, len(calls))
	var wg sync.WaitGroup
	wg.Add(len(calls))
	for i, c := range calls {
		i, c := i, c
		go func() {
			defer wg.Done()
			results[i] = invokeWithHooks(ctx, registry, c, uctx, opts)
		}()
	}
	wg.Wait()
	return results
}

// invokeWithHooks executes a single tool call with pre/post hooks.
func invokeWithHooks(
	ctx context.Context,
	registry *Registry,
	c CallRequest,
	uctx *UseContext,
	opts RunToolCallsOptions,
) CallResult {
	t := registry.Find(c.ToolName)
	if t == nil {
		return CallResult{
			ToolUseID: c.ToolUseID,
			Blocks:    ErrorResult(fmt.Sprintf("tool not found: %s", c.ToolName)),
			IsError:   true,
		}
	}

	input := c.Input

	// Pre-tool-use hook.
	if opts.HookExecutor != nil && opts.HookExecutor.HasHooksFor(hooks.EventPreToolUse) {
		hookInput := &hooks.HookInput{
			PreToolUse: &hooks.PreToolUseInput{
				ToolName: c.ToolName,
				ToolID:   c.ToolUseID,
				Input:    input,
			},
		}
		resp := opts.HookExecutor.RunSync(ctx, hooks.EventPreToolUse, hookInput)
		if resp.Decision == "block" {
			msg := "blocked by hook"
			if resp.FailureReason != "" {
				msg = resp.FailureReason
			}
			return CallResult{
				ToolUseID: c.ToolUseID,
				Blocks:    ErrorResult(msg),
				IsError:   true,
			}
		}
		if resp.UpdatedInput != nil {
			input = resp.UpdatedInput
		}
	}

	// Input validation.
	if err := t.ValidateInput(ctx, input); err != nil {
		return CallResult{
			ToolUseID: c.ToolUseID,
			Blocks:    ErrorResult("Invalid input: " + err.Error()),
			IsError:   true,
		}
	}

	// Permission check.
	if err := t.CheckPermissions(ctx, input, uctx); err != nil {
		// Fire PermissionDenied hook.
		if opts.HookExecutor != nil && opts.HookExecutor.HasHooksFor(hooks.EventPermissionDenied) {
			opts.HookExecutor.RunAsync(hooks.EventPermissionDenied, &hooks.HookInput{
				PermissionDenied: &hooks.PermissionDeniedInput{
					ToolName: c.ToolName,
					Reason:   err.Error(),
				},
			})
		}
		return CallResult{
			ToolUseID: c.ToolUseID,
			Blocks:    ErrorResult("Permission denied: " + err.Error()),
			IsError:   true,
		}
	}

	// Call the tool.
	blockCh, err := t.Call(ctx, input, uctx)
	if err != nil {
		return CallResult{
			ToolUseID: c.ToolUseID,
			Blocks:    ErrorResult(err.Error()),
			IsError:   true,
		}
	}

	var blocks []*engine.ContentBlock
	isErr := false
	for b := range blockCh {
		if b.IsError {
			isErr = true
		}
		blocks = append(blocks, b)
	}

	// Enforce max result size.
	maxSize := t.MaxResultSizeChars()
	if maxSize > 0 {
		blocks = truncateBlocks(blocks, maxSize)
	}

	// Post-tool-use hook.
	if opts.HookExecutor != nil && opts.HookExecutor.HasHooksFor(hooks.EventPostToolUse) {
		outputText := blocksToText(blocks)
		hookInput := &hooks.HookInput{
			PostToolUse: &hooks.PostToolUseInput{
				ToolName: c.ToolName,
				ToolID:   c.ToolUseID,
				Input:    input,
				Output:   outputText,
				IsError:  isErr,
			},
		}
		resp := opts.HookExecutor.RunSync(ctx, hooks.EventPostToolUse, hookInput)
		if resp.OutputOverride != nil {
			blocks = Result(*resp.OutputOverride)
			isErr = false
		}
	}

	// Capture context modifier from the tool.
	var ctxMod func(*engine.UseContext) *engine.UseContext
	if mod := t.ContextModifier(); mod != nil {
		ctxMod = mod
	}

	return CallResult{
		ToolUseID:       c.ToolUseID,
		Blocks:          blocks,
		IsError:         isErr,
		ContextModifier: ctxMod,
	}
}

// truncateBlocks limits the total character count of text blocks.
func truncateBlocks(blocks []*engine.ContentBlock, maxChars int) []*engine.ContentBlock {
	total := 0
	for _, b := range blocks {
		total += len(b.Text)
	}
	if total <= maxChars {
		return blocks
	}
	// Truncate: keep first maxChars characters across all blocks.
	remaining := maxChars
	var out []*engine.ContentBlock
	for _, b := range blocks {
		if remaining <= 0 {
			break
		}
		if len(b.Text) <= remaining {
			out = append(out, b)
			remaining -= len(b.Text)
		} else {
			truncated := *b
			truncated.Text = b.Text[:remaining] + "\n... [truncated]"
			out = append(out, &truncated)
			remaining = 0
		}
	}
	return out
}

// blocksToText concatenates text from all content blocks.
func blocksToText(blocks []*engine.ContentBlock) string {
	var buf []byte
	for _, b := range blocks {
		buf = append(buf, b.Text...)
	}
	return string(buf)
}
