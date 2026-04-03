package designsystem

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/wall-ai/agent-engine/internal/tui/color"
)

// BLOCKS is the Unicode block character set for sub-character progress rendering,
// matching claude-code-main's ProgressBar.tsx exactly.
var BLOCKS = []string{" ", "▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}

// RenderProgressBar renders a Unicode block progress bar.
//
// Parameters:
//   - ratio:      progress value between 0.0 and 1.0
//   - width:      total character width of the bar
//   - fillColor:  theme color string for the filled portion
//   - emptyColor: theme color string for the empty portion (background)
func RenderProgressBar(ratio float64, width int, fillColor, emptyColor string) string {
	if width <= 0 {
		return ""
	}

	// Clamp ratio
	ratio = math.Max(0, math.Min(1, ratio))

	whole := int(math.Floor(ratio * float64(width)))

	var segments []string

	// Filled portion
	if whole > 0 {
		segments = append(segments, strings.Repeat(BLOCKS[len(BLOCKS)-1], whole))
	}

	// Partial + empty portion
	if whole < width {
		remainder := ratio*float64(width) - float64(whole)
		middle := int(math.Floor(remainder * float64(len(BLOCKS))))
		if middle >= len(BLOCKS) {
			middle = len(BLOCKS) - 1
		}
		segments = append(segments, BLOCKS[middle])

		empty := width - whole - 1
		if empty > 0 {
			segments = append(segments, strings.Repeat(BLOCKS[0], empty))
		}
	}

	text := strings.Join(segments, "")

	fillStyle := lipgloss.NewStyle()
	if fillColor != "" {
		fillStyle = fillStyle.Foreground(color.Resolve(fillColor))
	}
	if emptyColor != "" {
		fillStyle = fillStyle.Background(color.Resolve(emptyColor))
	}

	return fillStyle.Render(text)
}
