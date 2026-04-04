package agent

import (
	"fmt"
	"strings"
)

// Agent prompt building aligned with claude-code-main's prompt.ts.
// Generates the AgentTool's prompt including the dynamic agent catalog
// and guidelines for writing effective agent prompts.

// BuildAgentToolPrompt generates the full prompt for the AgentTool.
// It includes the list of available agents and guidelines for using them.
func BuildAgentToolPrompt(agents []AgentDefinition, isCoordinatorMode bool) string {
	var sb strings.Builder

	sb.WriteString("Launch a new agent to handle complex, multi-step tasks autonomously.\n\n")

	// Agent catalog.
	if len(agents) > 0 {
		sb.WriteString("## Available Agent Types\n\n")
		for _, a := range agents {
			sb.WriteString(fmt.Sprintf("- **%s**", a.AgentType))
			if a.WhenToUse != "" {
				sb.WriteString(fmt.Sprintf(": %s", a.WhenToUse))
			}
			sb.WriteString("\n")
			if a.Model != "" {
				sb.WriteString(fmt.Sprintf("  Model: %s\n", a.Model))
			}
			if a.Background {
				sb.WriteString("  Default: runs in background\n")
			}
			if a.Isolation != IsolationNone {
				sb.WriteString(fmt.Sprintf("  Isolation: %s\n", a.Isolation))
			}
		}
		sb.WriteString("\n")
	}

	// When NOT to use.
	sb.WriteString("## When NOT to use the Task tool\n\n")
	sb.WriteString("- If you want to read a specific file path, use the Read tool or Glob tool instead\n")
	sb.WriteString("- If you are searching for a specific class definition like \"class Foo\", use Glob/Grep instead\n")
	sb.WriteString("- If you are searching for code within a specific file or set of 2-3 files, use the Read tool instead\n\n")

	// Fork guidance.
	if IsForkSubagentEnabled() {
		sb.WriteString("## When to fork\n\n")
		sb.WriteString("Use fork when you want to:\n")
		sb.WriteString("- Run multiple independent tasks in parallel from the current conversation context\n")
		sb.WriteString("- Each fork inherits the full conversation history for prompt cache efficiency\n")
		sb.WriteString("- Each fork works in an isolated git worktree\n\n")
	}

	// Usage notes.
	sb.WriteString("## Usage notes\n\n")
	sb.WriteString("- Always include a short description summarizing what the agent will do\n")
	sb.WriteString("- Launch multiple agents concurrently whenever possible to maximize performance\n")
	sb.WriteString("- When the agent is done, it returns a single result message. Summarize it for the user.\n")
	sb.WriteString("- Each agent invocation starts fresh — provide a complete task description\n")
	sb.WriteString("- The agent's outputs should generally be trusted\n")
	sb.WriteString("- Clearly tell the agent whether you expect it to write code or just do research\n\n")

	// Writing the prompt.
	sb.WriteString("## Writing the prompt\n\n")
	sb.WriteString("Brief the agent like a smart colleague who just walked into the room:\n")
	sb.WriteString("- Explain what you're trying to accomplish and why\n")
	sb.WriteString("- Describe what you've already learned or ruled out\n")
	sb.WriteString("- Give enough context that the agent can make judgment calls\n")
	sb.WriteString("- Be explicit about whether to write code or just research\n")

	if isCoordinatorMode {
		sb.WriteString("\n## Coordinator Mode\n\n")
		sb.WriteString("You are operating in coordinator mode. Your role is to:\n")
		sb.WriteString("1. Break complex tasks into independent work items\n")
		sb.WriteString("2. Spawn worker agents for each work item\n")
		sb.WriteString("3. Monitor progress and handle failures\n")
		sb.WriteString("4. Synthesize results when all workers complete\n")
		sb.WriteString("5. Use the shared scratchpad directory for coordination\n")
	}

	return sb.String()
}

// BuildAgentSystemPrompt generates the system prompt for a spawned agent.
// It combines the agent definition's system prompt with contextual information.
func BuildAgentSystemPrompt(def *AgentDefinition, parentContext *SubagentContext) string {
	if def.SystemPrompt != "" {
		return def.SystemPrompt
	}

	var sb strings.Builder
	sb.WriteString("You are a specialized agent working on a specific task.\n\n")

	if def.AgentType != "" {
		sb.WriteString(fmt.Sprintf("Agent type: %s\n", def.AgentType))
	}

	if def.WhenToUse != "" {
		sb.WriteString(fmt.Sprintf("Specialization: %s\n", def.WhenToUse))
	}

	if parentContext != nil && parentContext.TeamName != "" {
		sb.WriteString(fmt.Sprintf("Team: %s\n", parentContext.TeamName))
	}

	if def.CriticalSystemReminder != "" {
		sb.WriteString(fmt.Sprintf("\n## CRITICAL REMINDER\n%s\n", def.CriticalSystemReminder))
	}

	return sb.String()
}

// FormatAgentResult formats an agent's output for display to the parent.
func FormatAgentResult(result *AgentRunResult, maxChars int) string {
	if result.Error != nil {
		return fmt.Sprintf("Agent %s failed after %s (%d turns): %v",
			truncID(result.AgentID), result.Duration.Round(1e9),
			result.TurnCount, result.Error)
	}

	output := result.Output
	if maxChars > 0 && len(output) > maxChars {
		output = TruncateSubagentOutput(output, maxChars)
	}

	return fmt.Sprintf("Agent %s completed in %s (%d turns):\n\n%s",
		truncID(result.AgentID), result.Duration.Round(1e9),
		result.TurnCount, output)
}

// truncID returns the first 8 chars of an agent ID for display.
func truncID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
