package compact

import (
	"github.com/wall-ai/agent-engine/internal/engine"
)

// CollapseToolResults replaces the content of tool-result blocks that exceed
// maxChars with a shortened version, keeping the first and last portions.
func CollapseToolResults(messages []*engine.Message, maxChars int) []*engine.Message {
	if maxChars <= 0 {
		maxChars = 4000
	}

	out := make([]*engine.Message, 0, len(messages))
	for _, m := range messages {
		out = append(out, collapseMessage(m, maxChars))
	}
	return out
}

func collapseMessage(m *engine.Message, maxChars int) *engine.Message {
	newBlocks := make([]*engine.ContentBlock, 0, len(m.Content))
	for _, b := range m.Content {
		newBlocks = append(newBlocks, collapseBlock(b, maxChars))
	}
	return &engine.Message{
		ID:        m.ID,
		Role:      m.Role,
		Content:   newBlocks,
		Timestamp: m.Timestamp,
		SessionID: m.SessionID,
	}
}

func collapseBlock(b *engine.ContentBlock, maxChars int) *engine.ContentBlock {
	if b.Type != engine.ContentTypeToolResult {
		return b
	}
	// Collapse nested text blocks within the tool result.
	newContent := make([]*engine.ContentBlock, 0, len(b.Content))
	for _, c := range b.Content {
		if c.Type == engine.ContentTypeText && len(c.Text) > maxChars {
			head := c.Text[:maxChars/2]
			tail := c.Text[len(c.Text)-maxChars/2:]
			collapsed := head + "\n... [" +
				itoa(len(c.Text)-maxChars) + " chars collapsed] ...\n" + tail
			newContent = append(newContent, &engine.ContentBlock{
				Type: engine.ContentTypeText,
				Text: collapsed,
			})
		} else {
			newContent = append(newContent, c)
		}
	}
	return &engine.ContentBlock{
		Type:      b.Type,
		ToolUseID: b.ToolUseID,
		ToolName:  b.ToolName,
		Content:   newContent,
		IsError:   b.IsError,
	}
}

func itoa(n int) string {
	if n <= 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
