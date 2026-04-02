package engine

import (
	"context"
	"sync"
)

// ParallelToolExecutor runs multiple tool calls concurrently and collects
// all results.  Results are returned in the same order as the input requests.
type ParallelToolExecutor struct {
	executor *ToolExecutor
	// MaxConcurrency caps the number of goroutines.  0 means unbounded.
	MaxConcurrency int
}

// NewParallelToolExecutor wraps an existing ToolExecutor for concurrent use.
func NewParallelToolExecutor(exec *ToolExecutor, maxConcurrency int) *ParallelToolExecutor {
	return &ParallelToolExecutor{executor: exec, MaxConcurrency: maxConcurrency}
}

// ExecuteAll runs all requests concurrently and returns results in input order.
// If MaxConcurrency > 0 a semaphore limits active goroutines.
func (p *ParallelToolExecutor) ExecuteAll(ctx context.Context, reqs []*ToolExecRequest) []*ToolExecResult {
	if len(reqs) == 0 {
		return nil
	}
	if len(reqs) == 1 {
		return []*ToolExecResult{p.executor.Execute(ctx, reqs[0])}
	}

	results := make([]*ToolExecResult, len(reqs))
	var wg sync.WaitGroup

	var sem chan struct{}
	if p.MaxConcurrency > 0 {
		sem = make(chan struct{}, p.MaxConcurrency)
	}

	for i, req := range reqs {
		wg.Add(1)
		go func(idx int, r *ToolExecRequest) {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}
			results[idx] = p.executor.Execute(ctx, r)
		}(i, req)
	}
	wg.Wait()
	return results
}

// CanRunInParallel returns true when all tools in the batch are safe to run
// concurrently.  Currently a tool is considered parallel-safe unless it is
// a bash command (which may mutate shared state).
func CanRunInParallel(reqs []*ToolExecRequest) bool {
	for _, r := range reqs {
		if r.Tool.Name() == "Bash" || r.Tool.Name() == "bash" {
			return false
		}
	}
	return true
}
