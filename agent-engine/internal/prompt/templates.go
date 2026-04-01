package prompt

import _ "embed"

//go:embed embed/base_prompt.txt
var basePromptText string

//go:embed embed/undercover_instructions.txt
var undercoverInstructionsText string

// GetBasePrompt returns the compiled-in base system prompt.
// The embedded file can be overridden at build time by replacing base_prompt.txt.
func GetBasePrompt() string {
	return basePromptText
}

// GetUndercoverInstructions returns the undercover mode instructions injected
// into BashTool and commit/PR commands when undercover mode is active.
func GetUndercoverInstructions() string {
	return undercoverInstructionsText
}
