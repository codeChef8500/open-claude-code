package session

import (
	"fmt"
	"strings"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// ExportMarkdown converts a session transcript to a human-readable Markdown
// document.  It loads the transcript via ReadTranscript and formats each
// message as a Markdown section.
func (s *Storage) ExportMarkdown(sessionID string) (string, error) {
	entries, err := s.ReadTranscript(sessionID)
	if err != nil {
		return "", err
	}

	meta, _ := s.LoadMeta(sessionID)

	var sb strings.Builder
	sb.WriteString("# Session Transcript\n\n")
	if meta != nil {
		sb.WriteString(fmt.Sprintf("**Session ID:** %s  \n", meta.ID))
		sb.WriteString(fmt.Sprintf("**Work Dir:** %s  \n", meta.WorkDir))
		sb.WriteString(fmt.Sprintf("**Created:** %s  \n", meta.CreatedAt.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("**Turns:** %d  \n", meta.TurnCount))
		sb.WriteString(fmt.Sprintf("**Cost:** $%.6f  \n", meta.CostUSD))
		sb.WriteString("\n---\n\n")
	}

	for _, e := range entries {
		switch e.Type {
		case EntryTypeCompactSummary:
			sb.WriteString("## [Compact Summary]\n\n")
			if text, ok := e.Payload.(string); ok {
				sb.WriteString(text)
			}
			sb.WriteString("\n\n---\n\n")

		case EntryTypeMessage:
			msg, err := payloadToMessage(e.Payload)
			if err != nil {
				continue
			}
			role := strings.ToUpper(string(msg.Role))
			sb.WriteString(fmt.Sprintf("## %s\n\n", role))
			for _, block := range msg.Content {
				switch block.Type {
				case engine.ContentTypeText:
					sb.WriteString(block.Text)
					sb.WriteString("\n\n")
				case engine.ContentTypeToolUse:
					sb.WriteString(fmt.Sprintf("**Tool:** `%s`\n\n", block.ToolName))
				case engine.ContentTypeToolResult:
					sb.WriteString("**Tool Result:**\n\n")
					for _, c := range block.Content {
						if c.Type == engine.ContentTypeText {
							sb.WriteString("```\n")
							sb.WriteString(c.Text)
							sb.WriteString("\n```\n\n")
						}
					}
				case engine.ContentTypeThinking:
					sb.WriteString("<details><summary>Thinking</summary>\n\n")
					sb.WriteString(block.Thinking)
					sb.WriteString("\n\n</details>\n\n")
				}
			}
		}
	}

	return sb.String(), nil
}
