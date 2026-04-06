package askquestion

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ──────────────────────────────────────────────────────────────────────────────
// QuestionNavigationBar — Go port of QuestionNavigationBar.tsx
// Renders:  ← [☐ Auth] [☑ Library] [☐ Approach] ✓ Submit →
// ──────────────────────────────────────────────────────────────────────────────

// NavBarStyles controls the appearance of the navigation bar.
type NavBarStyles struct {
	ActiveTab   lipgloss.Style
	InactiveTab lipgloss.Style
	Arrow       lipgloss.Style
	DimArrow    lipgloss.Style
	Answered    lipgloss.Style
	Unanswered  lipgloss.Style
	SubmitTab   lipgloss.Style
}

// DefaultNavBarStyles returns sensible defaults.
func DefaultNavBarStyles() NavBarStyles {
	return NavBarStyles{
		ActiveTab:   lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("62")).Foreground(lipgloss.Color("15")).Padding(0, 1),
		InactiveTab: lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Padding(0, 1),
		Arrow:       lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		DimArrow:    lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true),
		Answered:    lipgloss.NewStyle().Foreground(lipgloss.Color("#4eba65")),
		Unanswered:  lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		SubmitTab:   lipgloss.NewStyle().Foreground(lipgloss.Color("#4eba65")).Bold(true).Padding(0, 1),
	}
}

// RenderNavBar renders the tab navigation bar.
//
//	questions       — the list of questions
//	currentIndex    — 0..len for current tab (len = submit page)
//	answers         — the answers map
//	hideSubmitTab   — whether to hide the submit tab
//	maxWidth        — terminal width to truncate into
func RenderNavBar(
	questions []Question,
	currentIndex int,
	answers map[string]string,
	hideSubmitTab bool,
	maxWidth int,
	styles NavBarStyles,
) string {
	if len(questions) <= 1 && hideSubmitTab {
		return ""
	}

	var parts []string

	// Left arrow
	if currentIndex > 0 {
		parts = append(parts, styles.Arrow.Render("←"))
	} else {
		parts = append(parts, styles.DimArrow.Render("←"))
	}

	// Question tabs
	for i, q := range questions {
		header := truncateHeader(q.Header, maxWidth, len(questions))

		// Checkbox indicator
		var indicator string
		if _, answered := answers[q.QuestionText]; answered {
			indicator = styles.Answered.Render("☑")
		} else {
			indicator = styles.Unanswered.Render("☐")
		}

		label := indicator + " " + header

		if i == currentIndex {
			parts = append(parts, styles.ActiveTab.Render(label))
		} else {
			parts = append(parts, styles.InactiveTab.Render(label))
		}
	}

	// Submit tab
	if !hideSubmitTab {
		submitLabel := "✓ Submit"
		if currentIndex == len(questions) {
			parts = append(parts, styles.ActiveTab.Render(submitLabel))
		} else {
			parts = append(parts, styles.SubmitTab.Render(submitLabel))
		}
	}

	// Right arrow
	maxIdx := len(questions)
	if hideSubmitTab {
		maxIdx = len(questions) - 1
	}
	if currentIndex < maxIdx {
		parts = append(parts, styles.Arrow.Render("→"))
	} else {
		parts = append(parts, styles.DimArrow.Render("→"))
	}

	return strings.Join(parts, " ")
}

// truncateHeader shortens a header to fit within the available width.
func truncateHeader(header string, termWidth, numQuestions int) string {
	// Estimate: each tab ≈ header + 6 chars (checkbox + padding + borders)
	// Plus arrows (4) + submit tab (~12) + spacing
	available := termWidth - 4 - 12 - (numQuestions * 6)
	perTab := available / max(numQuestions, 1)
	if perTab < 4 {
		perTab = 4
	}

	runes := []rune(header)
	if len(runes) <= perTab {
		return header
	}
	if perTab <= 1 {
		return string(runes[:1])
	}
	return string(runes[:perTab-1]) + "…"
}

// RenderNavBarCompact renders a compact "Question N of M" line when terminal is too narrow.
func RenderNavBarCompact(
	currentIndex int,
	totalQuestions int,
	isSubmitPage bool,
	styles NavBarStyles,
) string {
	if isSubmitPage {
		return styles.SubmitTab.Render("✓ Submit")
	}
	return fmt.Sprintf("Question %d of %d", currentIndex+1, totalQuestions)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
