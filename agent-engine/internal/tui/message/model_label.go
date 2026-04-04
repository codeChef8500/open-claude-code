package message

import (
	"github.com/charmbracelet/lipgloss"
)

// ModelLabel renders a model attribution line matching claude-code-main's
// MessageModel component. It only renders when the model changes between
// consecutive assistant messages (not on every message).
//
// Example output:  sonnet-4   (dim grey small text)
type ModelLabel struct {
	style lipgloss.Style
}

// NewModelLabel creates a model label renderer.
func NewModelLabel(dimColor lipgloss.TerminalColor) *ModelLabel {
	return &ModelLabel{
		style: lipgloss.NewStyle().Foreground(dimColor).Italic(true),
	}
}

// Render returns the model attribution string, or empty if it should be hidden.
// prevModel is the model used in the previous assistant message.
// currentModel is the model for this message.
// If they match (or currentModel is empty), returns "".
func (ml *ModelLabel) Render(prevModel, currentModel string) string {
	if currentModel == "" {
		return ""
	}
	if prevModel == currentModel {
		return ""
	}
	return ml.style.Render(currentModel)
}

// RenderAlways returns the model label regardless of previous model.
func (ml *ModelLabel) RenderAlways(model string) string {
	if model == "" {
		return ""
	}
	return ml.style.Render(model)
}

// ShortenModelName extracts a short display name from a full model identifier.
// e.g. "claude-sonnet-4-20250514" → "sonnet-4"
func ShortenModelName(model string) string {
	if model == "" {
		return ""
	}

	// Common patterns for claude models
	knownShorts := map[string]string{
		"claude-sonnet-4":   "sonnet-4",
		"claude-opus-4":     "opus-4",
		"claude-haiku-3.5":  "haiku-3.5",
		"claude-3-opus":     "opus-3",
		"claude-3-sonnet":   "sonnet-3",
		"claude-3-haiku":    "haiku-3",
		"claude-3.5-sonnet": "sonnet-3.5",
		"claude-3.5-haiku":  "haiku-3.5",
	}

	// Check prefix matches (handle date suffixes like -20250514)
	for prefix, short := range knownShorts {
		if len(model) >= len(prefix) && model[:len(prefix)] == prefix {
			return short
		}
	}

	// For non-Claude models, just return the original
	return model
}
