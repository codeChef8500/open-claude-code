package themes

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/wall-ai/agent-engine/internal/tui/color"
)

// ThemeName identifies a built-in theme.
type ThemeName string

const (
	ThemeDark            ThemeName = "dark"
	ThemeLight           ThemeName = "light"
	ThemeDarkAnsi        ThemeName = "dark-ansi"
	ThemeLightAnsi       ThemeName = "light-ansi"
	ThemeDarkDaltonized  ThemeName = "dark-daltonized"
	ThemeLightDaltonized ThemeName = "light-daltonized"
)

// ThemeNames returns all available theme names.
func ThemeNames() []ThemeName {
	return []ThemeName{
		ThemeDark, ThemeLight, ThemeDarkAnsi, ThemeLightAnsi,
		ThemeDarkDaltonized, ThemeLightDaltonized,
	}
}

// ResolveThemeSetting converts a setting string to a ThemeName.
func ResolveThemeSetting(setting string, systemDark bool) ThemeName {
	switch ThemeName(setting) {
	case ThemeDark, ThemeLight, ThemeDarkAnsi, ThemeLightAnsi,
		ThemeDarkDaltonized, ThemeLightDaltonized:
		return ThemeName(setting)
	default:
		if systemDark {
			return ThemeDark
		}
		return ThemeLight
	}
}

// IsDarkTheme returns true if the theme is a dark variant.
func IsDarkTheme(name ThemeName) bool {
	switch name {
	case ThemeDark, ThemeDarkAnsi, ThemeDarkDaltonized:
		return true
	}
	return false
}

// Theme holds all color values for a visual theme.
type Theme struct {
	Name                ThemeName
	AutoAccept          string
	BashBorder          string
	Claude              string
	ClaudeShimmer       string
	Permission          string
	PermissionShimmer   string
	PlanMode            string
	PromptBorder        string
	PromptBorderShimmer string
	Text                string
	InverseText         string
	Inactive            string
	InactiveShimmer     string
	Subtle              string
	Suggestion          string
	Background          string
	Success             string
	Error               string
	Warning             string
	WarningShimmer      string
	Merged              string
	DiffAdded           string
	DiffRemoved         string
	DiffAddedDimmed     string
	DiffRemovedDimmed   string
	DiffAddedWord       string
	DiffRemovedWord     string
	LobsterBody         string
	LobsterBackground   string
	UserMsgBg           string
	UserMsgBgHover      string
	BashMsgBg           string
	MemoryBg            string
	SelectionBg         string
	MsgActionsBg        string
	RateLimitFill       string
	RateLimitEmpty      string
	FastMode            string
	FastModeShimmer     string
	BriefLabelYou       string
	BriefLabelClaude    string
	AgentRed            string
	AgentBlue           string
	AgentGreen          string
	AgentYellow         string
	AgentPurple         string
	AgentOrange         string
	AgentPink           string
	AgentCyan           string
}

// C resolves a theme color string to a lipgloss.Color.
func (t Theme) C(field string) lipgloss.Color {
	return color.Resolve(field)
}
