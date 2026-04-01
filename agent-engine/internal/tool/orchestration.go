package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/wall-ai/agent-engine/internal/engine"
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
}

// RunToolCalls executes a batch of tool calls. Concurrency-safe tools are
// executed in parallel via goroutines; others run sequentially.
func RunToolCalls(
	ctx context.Context,
	registry *Registry,
	calls []CallRequest,
	uctx *UseContext,
) ([]CallResult, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	// Partition into safe (parallel) and unsafe (sequential) groups.
	var safeGroup, seqGroup []CallRequest
	for _, c := range calls {
		t := registry.Find(c.ToolName)
		if t != nil && t.IsConcurrencySafe() {
			safeGroup = append(safeGroup, c)
		} else {
			seqGroup = append(seqGroup, c)
		}
	}

	results := make([]CallResult, 0, len(calls))
	var mu sync.Mutex

	// Run safe group in parallel.
	if len(safeGroup) > 0 {
		var wg sync.WaitGroup
		wg.Add(len(safeGroup))
		for _, c := range safeGroup {
			c := c
			go func() {
				defer wg.Done()
				res := invoke(ctx, registry, c, uctx)
				mu.Lock()
				results = append(results, res)
				mu.Unlock()
			}()
		}
		wg.Wait()
	}

	// Run sequential group.
	for _, c := range seqGroup {
		results = append(results, invoke(ctx, registry, c, uctx))
	}

	return results, nil
}

func invoke(ctx context.Context, registry *Registry, c CallRequest, uctx *UseContext) CallResult {
	t := registry.Find(c.ToolName)
	if t == nil {
		return CallResult{
			ToolUseID: c.ToolUseID,
			Blocks:    ErrorResult(fmt.Sprintf("tool not found: %s", c.ToolName)),
			IsError:   true,
		}
	}

	// Permission check.
	if err := t.CheckPermissions(ctx, c.Input, uctx); err != nil {
		return CallResult{
			ToolUseID: c.ToolUseID,
			Blocks:    ErrorResult("Permission denied: " + err.Error()),
			IsError:   true,
		}
	}

	// Call the tool.
	blockCh, err := t.Call(ctx, c.Input, uctx)
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

	return CallResult{
		ToolUseID: c.ToolUseID,
		Blocks:    blocks,
		IsError:   isErr,
	}
}
