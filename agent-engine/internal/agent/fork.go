package agent

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/wall-ai/agent-engine/internal/engine"
)

// Fork subagent implementation aligned with claude-code-main's forkSubagent.ts.
//
// Fork subagents share the parent's conversation prefix for prompt cache hits.
// The key insight is that the forked messages must be byte-identical to the
// parent's API request prefix so the provider's prompt cache can be reused.

// IsForkSubagentEnabled checks if fork subagent functionality is available.
// Controlled by feature flag / environment variable.
func IsForkSubagentEnabled() bool {
	// TODO: Check feature flag / env var when available.
	return true
}

// IsInForkChild checks if the current context is already inside a fork child.
// Prevents recursive forking — fork children cannot fork again.
func IsInForkChild(parentContext *SubagentContext) bool {
	if parentContext == nil {
		return false
	}
	return parentContext.IsForkChild
}

// ForkPlaceholderResult is the byte-identical placeholder text used in
// tool_result blocks for all fork children. Every child gets the same
// placeholder so the API request prefix is byte-identical across siblings,
// maximizing prompt cache hits.
const ForkPlaceholderResult = "[Waiting for tool results — this agent is handling a different fork directive.]"

// BuildForkedMessages constructs the message array for a fork subagent.
// Aligned with claude-code-main's buildForkedMessages in forkSubagent.ts.
//
// The key design for prompt cache sharing:
//  1. Clone the parent's last assistant message (including all tool_use blocks)
//  2. Build placeholder tool_result blocks for every tool_use with IDENTICAL content
//  3. Append a per-child directive as the final text block
//
// This ensures all fork children have a byte-identical API request prefix,
// and only the very last text block differs — maximizing KV cache reuse.
func BuildForkedMessages(
	parentMessages []*engine.Message,
	childTask string,
	parentSystemPrompt string,
) []*engine.Message {
	if len(parentMessages) == 0 {
		// No parent context — fall back to a fresh conversation.
		return []*engine.Message{
			{
				Role: engine.RoleUser,
				Content: []*engine.ContentBlock{
					{Type: engine.ContentTypeText, Text: buildChildInstructions(childTask)},
				},
			},
		}
	}

	// Find the last assistant message with tool_use blocks.
	var lastAssistantIdx int = -1
	for i := len(parentMessages) - 1; i >= 0; i-- {
		if parentMessages[i].Role == engine.RoleAssistant {
			// Check if it has tool_use blocks.
			for _, cb := range parentMessages[i].Content {
				if cb.Type == engine.ContentTypeToolUse {
					lastAssistantIdx = i
					break
				}
			}
			if lastAssistantIdx >= 0 {
				break
			}
		}
	}

	// If no assistant message with tool_use found, fall back to simple clone.
	if lastAssistantIdx < 0 {
		forked := cloneMessages(parentMessages)
		forked = append(forked, buildChildMessage(childTask))
		return forked
	}

	// Clone messages up to and including the last assistant message.
	forked := cloneMessages(parentMessages[:lastAssistantIdx+1])

	// Collect all tool_use block IDs from the assistant message.
	var toolUseIDs []string
	for _, cb := range forked[lastAssistantIdx].Content {
		if cb.Type == engine.ContentTypeToolUse && cb.ToolUseID != "" {
			toolUseIDs = append(toolUseIDs, cb.ToolUseID)
		}
	}

	if len(toolUseIDs) == 0 {
		// No tool_use blocks — fall back to simple append.
		forked = append(forked, buildChildMessage(childTask))
		return forked
	}

	// Build the user message: placeholder tool_results + per-child directive.
	// All fork children get identical tool_result content, only the final
	// text block differs.
	var contentBlocks []*engine.ContentBlock

	// 1. Placeholder tool_result for each tool_use.
	for _, toolID := range toolUseIDs {
		contentBlocks = append(contentBlocks, &engine.ContentBlock{
			Type:      engine.ContentTypeToolResult,
			ToolUseID: toolID,
			Text:      ForkPlaceholderResult,
		})
	}

	// 2. Per-child directive as the final text block.
	contentBlocks = append(contentBlocks, &engine.ContentBlock{
		Type: engine.ContentTypeText,
		Text: buildChildInstructions(childTask),
	})

	toolResultMessage := &engine.Message{
		Role:    engine.RoleUser,
		Content: contentBlocks,
	}

	forked = append(forked, toolResultMessage)
	return forked
}

// cloneMessages creates a deep copy of a message slice.
func cloneMessages(msgs []*engine.Message) []*engine.Message {
	result := make([]*engine.Message, len(msgs))
	for i, m := range msgs {
		clone := *m
		// Deep copy content blocks.
		clone.Content = make([]*engine.ContentBlock, len(m.Content))
		for j, cb := range m.Content {
			blockClone := *cb
			clone.Content[j] = &blockClone
		}
		result[i] = &clone
	}
	return result
}

// buildChildMessage creates the user message injected into the fork child
// with strict instructions about output format and behavior.
func buildChildMessage(task string) *engine.Message {
	instructions := buildChildInstructions(task)
	return &engine.Message{
		Role: engine.RoleUser,
		Content: []*engine.ContentBlock{
			{Type: engine.ContentTypeText, Text: instructions},
		},
	}
}

// buildChildInstructions generates the instruction text for a fork child agent.
// Aligned with claude-code-main's buildChildMessage function.
func buildChildInstructions(task string) string {
	var sb strings.Builder

	sb.WriteString("You are a forked worker agent. You have been given a specific task to complete.\n\n")
	sb.WriteString("## Your Task\n\n")
	sb.WriteString(task)
	sb.WriteString("\n\n")
	sb.WriteString("## Rules\n\n")
	sb.WriteString("1. Focus ONLY on the task described above.\n")
	sb.WriteString("2. Do NOT ask questions or seek clarification — make your best judgment.\n")
	sb.WriteString("3. Do NOT delegate to other agents — complete the work yourself.\n")
	sb.WriteString("4. When you are done, provide a clear summary of what you did.\n")
	sb.WriteString("5. If you encounter an error you cannot resolve, explain what went wrong.\n")
	sb.WriteString("6. Work in your assigned worktree — do not modify files outside it.\n")

	return sb.String()
}

// ForkAgentParams builds RunAgentParams for a fork subagent.
func ForkAgentParams(
	task string,
	parentMessages []*engine.Message,
	parentSystemPrompt string,
	workDir string,
	parentContext *SubagentContext,
) RunAgentParams {
	forkDef := ForkAgent // copy
	forkDef.AgentID = uuid.New().String()

	// Build forked messages with prompt cache sharing.
	forkedMessages := BuildForkedMessages(parentMessages, task, parentSystemPrompt)

	return RunAgentParams{
		AgentDef:       &forkDef,
		Task:           task,
		ParentMessages: forkedMessages,
		ParentContext:  parentContext,
		WorkDir:        workDir,
		IsFork:         true,
		IsolationMode:  IsolationWorktree,
		Background:     true,
	}
}

// ShouldFork determines if the AgentTool should use fork mode.
// Fork is preferred when:
//  1. Fork subagent is enabled
//  2. Not already inside a fork child (no recursive forks)
//  3. Parent has conversation context to share
//  4. Agent runs in background with worktree isolation
func ShouldFork(
	parentContext *SubagentContext,
	parentMessages []*engine.Message,
	isolation IsolationMode,
	background bool,
) bool {
	if !IsForkSubagentEnabled() {
		return false
	}
	if IsInForkChild(parentContext) {
		return false
	}
	if len(parentMessages) == 0 {
		return false
	}
	// Fork requires worktree isolation and background execution.
	return isolation == IsolationWorktree && background
}

// SummarizeForkResult creates a compact summary of a fork agent's output.
func SummarizeForkResult(result *AgentRunResult) string {
	if result.Error != nil {
		return fmt.Sprintf("Fork agent %s failed: %v", result.AgentID[:8], result.Error)
	}

	output := result.Output
	if len(output) > 2000 {
		output = TruncateSubagentOutput(output, 2000)
	}

	return fmt.Sprintf("Fork agent %s completed in %s:\n%s",
		result.AgentID[:8],
		result.Duration.Round(1e9), // round to seconds
		output,
	)
}
