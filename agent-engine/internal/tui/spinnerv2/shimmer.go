package spinnerv2

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/wall-ai/agent-engine/internal/tui/color"
)

// ShimmerText renders a string with a per-character color wave effect,
// oscillating between baseColor and shimmerColor. This replicates the
// GlimmerMessage shimmer animation from claude-code-main.
//
// Parameters:
//   - text:         the label string to render
//   - baseColor:    theme color string for the dim state (e.g. theme.Claude)
//   - shimmerColor: theme color string for the bright state (e.g. theme.ClaudeShimmer)
//   - timeMs:       elapsed milliseconds since animation started
func ShimmerText(text string, baseColor, shimmerColor string, timeMs int64) string {
	baseRGB, baseOk := color.ParseRGB(baseColor)
	shimRGB, shimOk := color.ParseRGB(shimmerColor)

	// Fallback: if colors aren't parseable, render with base color only
	if !baseOk || !shimOk {
		style := lipgloss.NewStyle().Foreground(color.Resolve(baseColor))
		return style.Render(text)
	}

	var sb strings.Builder
	runes := []rune(text)

	for i, ch := range runes {
		// Wave function: each character gets a phase offset based on its index.
		// Period ~80ms per character offset, full cycle ~2π.
		phase := float64(timeMs)/400.0 + float64(i)*0.3
		// Map sine [-1,1] to [0,1]
		t := (math.Sin(phase) + 1.0) / 2.0

		blended := color.Interpolate(baseRGB, shimRGB, t)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(blended.ToHex()))
		sb.WriteString(style.Render(string(ch)))
	}

	return sb.String()
}
