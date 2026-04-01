package prompt

import (
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// ProcessedInput is the result of pre-processing a raw user input string.
type ProcessedInput struct {
	// Text with @file mentions expanded.
	Text    string
	// Image blocks extracted from the input or passed in separately.
	Images  []*engine.ContentBlock
	// Slash command if the text starts with '/'.
	Command string
	// Arguments after the command name.
	CommandArgs []string
}

// ProcessUserInput normalises a raw user input string:
//  1. Detects slash commands (/compact, /clear, etc.)
//  2. Expands @file mentions
//  3. Attaches any explicitly provided image blocks
func ProcessUserInput(raw string, workDir string, images []*engine.ContentBlock) *ProcessedInput {
	raw = strings.TrimSpace(raw)
	result := &ProcessedInput{Images: images}

	// Detect slash commands.
	if strings.HasPrefix(raw, "/") {
		parts := strings.Fields(raw)
		if len(parts) > 0 {
			result.Command = parts[0][1:] // strip leading /
			result.CommandArgs = parts[1:]
			result.Text = raw
			return result
		}
	}

	// Expand @file mentions.
	result.Text = ExpandFileMentions(raw, workDir)
	return result
}

// BuildContentBlocks converts a ProcessedInput into engine ContentBlocks
// for inclusion in a Message.
func BuildContentBlocks(pi *ProcessedInput) []*engine.ContentBlock {
	var blocks []*engine.ContentBlock
	if pi.Text != "" {
		blocks = append(blocks, &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: pi.Text,
		})
	}
	blocks = append(blocks, pi.Images...)
	return blocks
}
