package logo

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/wall-ai/agent-engine/internal/tui/color"
	"github.com/wall-ai/agent-engine/internal/tui/themes"
)

// BannerData holds the dynamic information shown in the welcome banner.
type BannerData struct {
	Version string
	Model   string
	Billing string // "API" / "Pro" / etc.
	CWD     string
	Agent   string // optional agent/teammate name
}

// RenderCondensedBanner renders the compact startup banner wrapped in a
// rounded border with the Claude theme color, matching claude-code-main's
// CondensedLogo component:
//
//	╭──────────────────────────────────────╮
//	│ [Clawd]  Claude Code v1.0.0         │
//	│          sonnet-4 · API              │
//	│          /path/to/cwd                │
//	╰──────────────────────────────────────╯
func RenderCondensedBanner(data BannerData, theme themes.Theme, width int) string {
	content := renderBannerContent(data, theme, width)

	c := func(s string) lipgloss.Color { return color.Resolve(s) }
	borderBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c(theme.Claude)).
		Width(width - 2)

	return borderBox.Render(content)
}

// renderBannerContent builds the inner content of the banner (Clawd + info).
func renderBannerContent(data BannerData, theme themes.Theme, width int) string {
	clawd := RenderClawd(PoseDefault, theme)
	clawdLines := strings.Split(clawd, "\n")

	c := func(s string) lipgloss.Color { return color.Resolve(s) }
	titleStyle := lipgloss.NewStyle().Foreground(c(theme.Claude)).Bold(true)
	dimStyle := lipgloss.NewStyle().Faint(true)
	subtleStyle := lipgloss.NewStyle().Foreground(c(theme.Inactive))

	// Info lines (right of the mascot)
	var infoLines []string

	// Line 1: title + version
	title := titleStyle.Render("Claude Code")
	if data.Version != "" {
		title += dimStyle.Render(" v" + data.Version)
	}
	infoLines = append(infoLines, title)

	// Line 2: model · billing
	var modelParts []string
	if data.Model != "" {
		modelParts = append(modelParts, data.Model)
	}
	if data.Billing != "" {
		modelParts = append(modelParts, data.Billing)
	}
	if len(modelParts) > 0 {
		infoLines = append(infoLines, subtleStyle.Render(strings.Join(modelParts, " · ")))
	}

	// Line 3: agent (optional) · cwd
	if data.CWD != "" {
		var cwdLine string
		if data.Agent != "" {
			cwdLine = "@" + data.Agent + " · " + shortenCWD(data.CWD, width-20-len(data.Agent))
		} else {
			cwdLine = shortenCWD(data.CWD, width-14)
		}
		infoLines = append(infoLines, subtleStyle.Render(cwdLine))
	}

	// Compose: clawd lines on the left, info lines on the right
	gap := "  " // 2 spaces between mascot and info
	var result []string
	maxLines := len(clawdLines)
	if len(infoLines) > maxLines {
		maxLines = len(infoLines)
	}

	clawdWidth := 10 // approximate width of clawd mascot
	for i := 0; i < maxLines; i++ {
		left := ""
		if i < len(clawdLines) {
			left = clawdLines[i]
		}
		// Pad left to uniform width
		leftPad := clawdWidth - lipgloss.Width(left)
		if leftPad < 0 {
			leftPad = 0
		}
		left += strings.Repeat(" ", leftPad)

		right := ""
		if i < len(infoLines) {
			right = infoLines[i]
		}
		result = append(result, left+gap+right)
	}

	return strings.Join(result, "\n")
}

// RenderFullBanner renders the full welcome banner with a welcome message
// above the condensed banner, matching claude-code-main's LogoV2 component.
func RenderFullBanner(data BannerData, theme themes.Theme, width int) string {
	c := func(s string) lipgloss.Color { return color.Resolve(s) }

	// Welcome line
	welcomeStyle := lipgloss.NewStyle().Foreground(c(theme.Text)).Bold(true)
	welcome := welcomeStyle.Render("Welcome to Claude Code!")

	content := renderBannerContent(data, theme, width)

	// Combine welcome + content inside rounded border
	fullContent := welcome + "\n\n" + content

	borderBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c(theme.Claude)).
		Width(width - 2).
		PaddingLeft(1).PaddingRight(1)

	return borderBox.Render(fullContent)
}

// shortenCWD shortens a working directory path if it exceeds maxLen.
func shortenCWD(cwd string, maxLen int) string {
	if maxLen < 10 {
		maxLen = 10
	}
	if len(cwd) <= maxLen {
		return cwd
	}
	// Try using just the last component
	base := filepath.Base(cwd)
	parent := filepath.Base(filepath.Dir(cwd))
	short := filepath.Join("…", parent, base)
	if len(short) <= maxLen {
		return short
	}
	return "…" + cwd[len(cwd)-maxLen+1:]
}
