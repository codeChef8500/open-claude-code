package compact

import "strings"

// Compact prompt templates aligned with claude-code-main compact/prompt.ts.

const noToolsPreamble = `CRITICAL: Respond with TEXT ONLY. Do NOT call any tools.

- Do NOT use Read, Bash, Grep, Glob, Edit, Write, or ANY other tool.
- You already have all the context you need in the conversation above.
- Tool calls will be REJECTED and will waste your only turn — you will fail the task.
- Your entire response must be plain text: an <analysis> block followed by a <summary> block.

`

const detailedAnalysisInstruction = `Before providing your final summary, wrap your analysis in <analysis> tags to organize your thoughts and ensure you've covered all necessary points. In your analysis process:

1. Chronologically analyze each message and section of the conversation. For each section thoroughly identify:
   - The user's explicit requests and intents
   - Your approach to addressing the user's requests
   - Key decisions, technical concepts and code patterns
   - Specific details like:
     - file names
     - full code snippets
     - function signatures
     - file edits
   - Errors that you ran into and how you fixed them
   - Pay special attention to specific user feedback that you received, especially if the user told you to do something differently.
2. Double-check for technical accuracy and completeness, addressing each required element thoroughly.`

const baseCompactPrompt = `Your task is to create a detailed summary of the conversation so far, paying close attention to the user's explicit requests and your previous actions.
This summary should be thorough in capturing technical details, code patterns, and architectural decisions that would be essential for continuing development work without losing context.

` + detailedAnalysisInstruction + `

Your summary should include the following sections:

1. Primary Request and Intent: Capture all of the user's explicit requests and intents in detail
2. Key Technical Concepts: List all important technical concepts, technologies, and frameworks discussed.
3. Files and Code Sections: Enumerate specific files and code sections examined, modified, or created. Pay special attention to the most recent messages and include full code snippets where applicable and include a summary of why this file read or edit is important.
4. Errors and fixes: List all errors that you ran into, and how you fixed them. Pay special attention to specific user feedback that you received, especially if the user told you to do something differently.
5. Problem Solving: Document problems solved and any ongoing troubleshooting efforts.
6. All user messages: List ALL user messages that are not tool results. These are critical for understanding the users' feedback and changing intent.
7. Pending Tasks: Outline any pending tasks that you have explicitly been asked to work on.
8. Current Work: Describe in detail precisely what was being worked on immediately before this summary request, paying special attention to the most recent messages from both user and assistant. Include file names and code snippets where applicable.
9. Optional Next Step: List the next step that you will take that is related to the most recent work you were doing. IMPORTANT: ensure that this step is DIRECTLY in line with the user's most recent explicit requests, and the task you were working on immediately before this summary request. If your last task was concluded, then only list next steps if they are explicitly in line with the users request. Do not start on tangential requests or really old requests that were already completed without confirming with the user first.
                       If there is a next step, include direct quotes from the most recent conversation showing exactly what task you were working on and where you left off. This should be verbatim to ensure there's no drift in task interpretation.

Please provide your summary based on the conversation so far, following this structure and ensuring precision and thoroughness in your response.`

// GetCompactPrompt returns the full compact prompt, optionally appending
// custom instructions provided by the user or hooks.
func GetCompactPrompt(customInstructions string) string {
	prompt := noToolsPreamble + baseCompactPrompt
	if customInstructions != "" {
		prompt += "\n\nAdditional summarization instructions:\n" + customInstructions
	}
	return prompt
}

// GetCompactSystemPrompt returns the system prompt for the compact LLM call.
func GetCompactSystemPrompt() string {
	return `You are a conversation summariser.
Produce a concise but complete summary of the conversation so far, preserving all key decisions, 
file paths, code changes, and outstanding tasks. The summary will replace the full history 
to free up context window space.
Follow the user's instructions for summary structure exactly.`
}

// FormatCompactSummary extracts the <summary> block from the LLM response,
// stripping the <analysis> drafting scratchpad. If no <summary> tags are
// found, the entire response is returned.
func FormatCompactSummary(raw string) string {
	const startTag = "<summary>"
	const endTag = "</summary>"

	startIdx := strings.Index(raw, startTag)
	endIdx := strings.LastIndex(raw, endTag)

	if startIdx >= 0 && endIdx > startIdx {
		return strings.TrimSpace(raw[startIdx+len(startTag) : endIdx])
	}
	// Fallback: strip <analysis> block if present.
	if aStart := strings.Index(raw, "<analysis>"); aStart >= 0 {
		if aEnd := strings.Index(raw, "</analysis>"); aEnd > aStart {
			before := raw[:aStart]
			after := raw[aEnd+len("</analysis>"):]
			return strings.TrimSpace(before + after)
		}
	}
	return strings.TrimSpace(raw)
}

// MergeHookInstructions merges user-supplied custom instructions with
// hook-provided instructions. User instructions come first.
func MergeHookInstructions(userInstructions, hookInstructions string) string {
	if hookInstructions == "" {
		return userInstructions
	}
	if userInstructions == "" {
		return hookInstructions
	}
	return userInstructions + "\n\n" + hookInstructions
}
