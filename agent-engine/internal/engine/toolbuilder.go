package engine

import (
	"encoding/json"
	"strings"
)

// ToolMatchesName reports whether t matches name by primary name or alias.
// Aligned with claude-code-main's toolMatchesName (Tool.ts:348-353).
func ToolMatchesName(t Tool, name string) bool {
	if strings.EqualFold(t.Name(), name) {
		return true
	}
	for _, a := range t.Aliases() {
		if strings.EqualFold(a, name) {
			return true
		}
	}
	return false
}

// FindToolByName returns the first tool whose name or alias matches.
// Aligned with claude-code-main's findToolByName (Tool.ts:358-360).
func FindToolByName(tools []Tool, name string) Tool {
	for _, t := range tools {
		if ToolMatchesName(t, name) {
			return t
		}
	}
	return nil
}

// FilterEnabledTools returns only those tools that pass IsEnabled.
func FilterEnabledTools(tools []Tool, uctx *UseContext) []Tool {
	var out []Tool
	for _, t := range tools {
		if t.IsEnabled(uctx) {
			out = append(out, t)
		}
	}
	return out
}

// ToolsToDefs converts a slice of Tools to ToolDefinitions for the API.
func ToolsToDefs(tools []Tool) []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(tools))
	for _, t := range tools {
		defs = append(defs, ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return defs
}

// PartitionToolCalls partitions tool_use blocks into concurrency-safe batches.
// Aligned with claude-code-main's partitionToolCalls (toolOrchestration.ts).
//
// Rules:
//   - Calls whose tool reports IsConcurrencySafe(input)=true are grouped
//     into a single concurrent batch.
//   - Everything else is sequential (one batch per call).
//   - Within each batch, order is preserved.
type ToolBatch struct {
	Calls             []ToolCallBlock
	IsConcurrencySafe bool
}

// ToolCallBlock is a lightweight reference to a tool_use content block
// used during orchestration.
type ToolCallBlock struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// PartitionToolCalls splits calls into batches using the provided tool registry.
func PartitionToolCalls(calls []ToolCallBlock, tools []Tool) []ToolBatch {
	if len(calls) == 0 {
		return nil
	}

	var batches []ToolBatch
	var concurrentBatch []ToolCallBlock

	flush := func() {
		if len(concurrentBatch) > 0 {
			batches = append(batches, ToolBatch{
				Calls:             concurrentBatch,
				IsConcurrencySafe: true,
			})
			concurrentBatch = nil
		}
	}

	for _, c := range calls {
		t := FindToolByName(tools, c.Name)
		if t != nil && t.IsConcurrencySafe(c.Input) {
			concurrentBatch = append(concurrentBatch, c)
		} else {
			// Flush any pending concurrent batch, then add this as sequential.
			flush()
			batches = append(batches, ToolBatch{
				Calls:             []ToolCallBlock{c},
				IsConcurrencySafe: false,
			})
		}
	}
	flush()

	return batches
}

// DeferredToolDefs returns ToolDefinitions with defer_loading markers for
// tools that should be deferred (ShouldDefer=true), excluding them from
// the initial prompt when ToolSearch is available.
func DeferredToolDefs(tools []Tool, hasToolSearch bool) (active, deferred []ToolDefinition) {
	for _, t := range tools {
		def := ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		}
		if hasToolSearch && t.ShouldDefer() && !t.AlwaysLoad() {
			deferred = append(deferred, def)
		} else {
			active = append(active, def)
		}
	}
	return
}
