package askquestion

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/ansi"
)

// ──────────────────────────────────────────────────────────────────────────────
// PreviewBox — Go port of PreviewBox.tsx
// Renders a bordered monospace box with optional truncation.
//
//	┌────────────────────────────────┐
//	│ content line 1                 │
//	│ content line 2                 │
//	├──── ✂ ──── 5 lines hidden ────┤
//	└────────────────────────────────┘
// ──────────────────────────────────────────────────────────────────────────────

// PreviewBoxStyles controls the appearance.
type PreviewBoxStyles struct {
	Border   lipgloss.Style
	Content  lipgloss.Style
	Truncate lipgloss.Style
}

// DefaultPreviewBoxStyles returns sensible defaults.
func DefaultPreviewBoxStyles() PreviewBoxStyles {
	return PreviewBoxStyles{
		Border:   lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Content:  lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		Truncate: lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true),
	}
}

// PreviewBoxOpts controls the box dimensions.
type PreviewBoxOpts struct {
	Content   string
	MaxLines  int // 0 = no limit
	MinHeight int // minimum visible lines
	MinWidth  int
	MaxWidth  int
}

// RenderPreviewBox renders a bordered monospace preview box.
func RenderPreviewBox(opts PreviewBoxOpts, styles PreviewBoxStyles) string {
	if opts.MaxWidth < 10 {
		opts.MaxWidth = 60
	}
	if opts.MinWidth < 10 {
		opts.MinWidth = 20
	}

	// Split content into lines
	lines := strings.Split(opts.Content, "\n")

	// Calculate box width from content
	boxWidth := opts.MinWidth
	for _, line := range lines {
		w := ansi.PrintableRuneWidth(line)
		if w+4 > boxWidth { // +4 for "│ " + " │"
			boxWidth = w + 4
		}
	}
	if boxWidth > opts.MaxWidth {
		boxWidth = opts.MaxWidth
	}

	innerWidth := boxWidth - 4 // space for "│ " + " │"
	if innerWidth < 1 {
		innerWidth = 1
	}

	// Truncate if needed
	truncated := 0
	visibleLines := lines
	if opts.MaxLines > 0 && len(lines) > opts.MaxLines {
		truncated = len(lines) - opts.MaxLines
		visibleLines = lines[:opts.MaxLines]
	}

	// Pad to minHeight
	for len(visibleLines) < opts.MinHeight {
		visibleLines = append(visibleLines, "")
	}

	var sb strings.Builder
	borderStyle := styles.Border

	// Top border
	sb.WriteString(borderStyle.Render("┌" + strings.Repeat("─", boxWidth-2) + "┐"))
	sb.WriteString("\n")

	// Content lines
	for _, line := range visibleLines {
		paddedLine := padOrTruncateLine(line, innerWidth)
		sb.WriteString(borderStyle.Render("│") + " " + styles.Content.Render(paddedLine) + " " + borderStyle.Render("│"))
		sb.WriteString("\n")
	}

	// Truncation indicator
	if truncated > 0 {
		truncMsg := fmt.Sprintf(" ✂ %d lines hidden ", truncated)
		dashesNeeded := boxWidth - 2 - printableLen(truncMsg)
		leftDashes := dashesNeeded / 2
		rightDashes := dashesNeeded - leftDashes
		if leftDashes < 1 {
			leftDashes = 1
		}
		if rightDashes < 1 {
			rightDashes = 1
		}
		truncLine := "├" + strings.Repeat("─", leftDashes) + styles.Truncate.Render(truncMsg) + strings.Repeat("─", rightDashes) + "┤"
		sb.WriteString(borderStyle.Render(truncLine))
		sb.WriteString("\n")
	}

	// Bottom border
	sb.WriteString(borderStyle.Render("└" + strings.Repeat("─", boxWidth-2) + "┘"))

	return sb.String()
}

// padOrTruncateLine pads a line to exactly `width` printable characters,
// or truncates it with "…" if too long.
func padOrTruncateLine(line string, width int) string {
	pw := ansi.PrintableRuneWidth(line)
	if pw <= width {
		return line + strings.Repeat(" ", width-pw)
	}
	// Truncate: remove chars from the end until it fits
	runes := []rune(line)
	for len(runes) > 0 {
		candidate := string(runes) + "…"
		if ansi.PrintableRuneWidth(candidate) <= width {
			return candidate + strings.Repeat(" ", width-ansi.PrintableRuneWidth(candidate))
		}
		runes = runes[:len(runes)-1]
	}
	return strings.Repeat(" ", width)
}

func printableLen(s string) int {
	return ansi.PrintableRuneWidth(s)
}
