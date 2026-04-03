package statusline

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// StatusData holds all data displayed in the status line.
type StatusData struct {
	Model          string
	Provider       string
	CostUSD        float64
	InputTokens    int
	OutputTokens   int
	CacheReadTokens int
	ContextPct     float64  // 0.0 - 1.0
	PermissionMode string
	WorkDir        string
	SessionName    string
	TurnCount      int
	IsLoading      bool
	LoadingStart   time.Time
	ToolActive     string   // name of active tool, if any
}

// Theme holds styles for the status line.
type Theme struct {
	Background lipgloss.Style
	Model      lipgloss.Style
	Cost       lipgloss.Style
	Context    lipgloss.Style
	Mode       lipgloss.Style
	Path       lipgloss.Style
	Dim        lipgloss.Style
	Warning    lipgloss.Style
	Active     lipgloss.Style
}

// DefaultDarkTheme returns a dark-mode status line theme.
func DefaultDarkTheme() Theme {
	bg := lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("250"))
	return Theme{
		Background: bg.Padding(0, 1),
		Model:      bg.Foreground(lipgloss.Color("15")).Bold(true),
		Cost:       bg.Foreground(lipgloss.Color("10")),
		Context:    bg.Foreground(lipgloss.Color("14")),
		Mode:       bg.Foreground(lipgloss.Color("11")),
		Path:       bg.Foreground(lipgloss.Color("240")),
		Dim:        bg.Foreground(lipgloss.Color("240")),
		Warning:    bg.Foreground(lipgloss.Color("9")).Bold(true),
		Active:     bg.Foreground(lipgloss.Color("205")),
	}
}

// Render builds the full status line string.
func Render(data StatusData, width int, theme Theme) string {
	// ── Left section: model · cost ──
	var leftParts []string

	leftParts = append(leftParts, theme.Model.Render(data.Model))

	if data.CostUSD > 0 {
		leftParts = append(leftParts, theme.Cost.Render(formatCost(data.CostUSD)))
	}

	// Context usage bar
	if data.ContextPct > 0 {
		bar := renderContextBar(data.ContextPct, 10, theme)
		leftParts = append(leftParts, bar)
	}

	left := strings.Join(leftParts, theme.Dim.Render(" · "))

	// ── Right section: mode · cwd ──
	var rightParts []string

	if data.IsLoading && data.ToolActive != "" {
		elapsed := time.Since(data.LoadingStart).Truncate(time.Second)
		rightParts = append(rightParts, theme.Active.Render(fmt.Sprintf("⚙ %s (%s)", data.ToolActive, elapsed)))
	} else if data.IsLoading {
		elapsed := time.Since(data.LoadingStart).Truncate(time.Second)
		rightParts = append(rightParts, theme.Active.Render(fmt.Sprintf("thinking (%s)", elapsed)))
	}

	if data.PermissionMode != "" && data.PermissionMode != "default" {
		rightParts = append(rightParts, theme.Mode.Render(data.PermissionMode))
	}

	if data.TurnCount > 0 {
		rightParts = append(rightParts, theme.Dim.Render(fmt.Sprintf("turn %d", data.TurnCount)))
	}

	if data.WorkDir != "" {
		rightParts = append(rightParts, theme.Path.Render(shortenPath(data.WorkDir, 25)))
	}

	right := strings.Join(rightParts, theme.Dim.Render(" · "))

	// ── Compose ──
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gap := width - leftW - rightW - 2
	if gap < 1 {
		gap = 1
	}

	return theme.Background.Width(width).Render(left + strings.Repeat(" ", gap) + right)
}

// renderContextBar builds a mini progress bar for context usage.
func renderContextBar(pct float64, barWidth int, theme Theme) string {
	if pct > 1.0 {
		pct = 1.0
	}
	filled := int(pct * float64(barWidth))
	empty := barWidth - filled

	var bar string
	if pct > 0.9 {
		bar = theme.Warning.Render(strings.Repeat("█", filled))
	} else if pct > 0.7 {
		bar = theme.Mode.Render(strings.Repeat("█", filled))
	} else {
		bar = theme.Context.Render(strings.Repeat("█", filled))
	}
	bar += theme.Dim.Render(strings.Repeat("░", empty))

	label := fmt.Sprintf(" %d%%", int(pct*100))
	return bar + theme.Dim.Render(label)
}

// formatCost formats USD cost for display.
func formatCost(usd float64) string {
	if usd < 0.01 {
		return fmt.Sprintf("$%.4f", usd)
	}
	return fmt.Sprintf("$%.2f", usd)
}

// shortenPath shortens a path for status line display.
func shortenPath(p string, maxLen int) string {
	if len(p) <= maxLen {
		return p
	}
	return "…" + p[len(p)-maxLen+1:]
}
