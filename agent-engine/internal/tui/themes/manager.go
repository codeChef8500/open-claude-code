package themes

import "github.com/charmbracelet/lipgloss"

// ThemeName identifies a built-in theme.
type ThemeName string

const (
	ThemeDark      ThemeName = "dark"
	ThemeLight     ThemeName = "light"
	ThemeMonokai   ThemeName = "monokai"
	ThemeSolarized ThemeName = "solarized"
	ThemeNord      ThemeName = "nord"
)

// ColorScheme holds all colors for a theme.
type ColorScheme struct {
	Name        ThemeName
	Background  lipgloss.Color
	Foreground  lipgloss.Color
	Primary     lipgloss.Color
	Secondary   lipgloss.Color
	Accent      lipgloss.Color
	Success     lipgloss.Color
	Warning     lipgloss.Color
	Error       lipgloss.Color
	Dim         lipgloss.Color
	Border      lipgloss.Color
	UserColor   lipgloss.Color
	AssistColor lipgloss.Color
	SystemColor lipgloss.Color
	CodeBg      lipgloss.Color
	CodeFg      lipgloss.Color
	DiffAdd     lipgloss.Color
	DiffDel     lipgloss.Color
	ToolColor   lipgloss.Color
	Highlight   lipgloss.Color
}

// BuiltinThemes returns all available themes.
func BuiltinThemes() map[ThemeName]ColorScheme {
	return map[ThemeName]ColorScheme{
		ThemeDark:      darkScheme(),
		ThemeLight:     lightScheme(),
		ThemeMonokai:   monokaiScheme(),
		ThemeSolarized: solarizedScheme(),
		ThemeNord:      nordScheme(),
	}
}

// GetTheme returns a named theme, falling back to dark.
func GetTheme(name ThemeName) ColorScheme {
	themes := BuiltinThemes()
	if t, ok := themes[name]; ok {
		return t
	}
	return themes[ThemeDark]
}

// ThemeNames returns the available theme names.
func ThemeNames() []ThemeName {
	return []ThemeName{ThemeDark, ThemeLight, ThemeMonokai, ThemeSolarized, ThemeNord}
}

func darkScheme() ColorScheme {
	return ColorScheme{
		Name:        ThemeDark,
		Background:  lipgloss.Color("0"),
		Foreground:  lipgloss.Color("250"),
		Primary:     lipgloss.Color("12"),
		Secondary:   lipgloss.Color("14"),
		Accent:      lipgloss.Color("205"),
		Success:     lipgloss.Color("10"),
		Warning:     lipgloss.Color("11"),
		Error:       lipgloss.Color("9"),
		Dim:         lipgloss.Color("240"),
		Border:      lipgloss.Color("240"),
		UserColor:   lipgloss.Color("12"),
		AssistColor: lipgloss.Color("10"),
		SystemColor: lipgloss.Color("11"),
		CodeBg:      lipgloss.Color("235"),
		CodeFg:      lipgloss.Color("15"),
		DiffAdd:     lipgloss.Color("10"),
		DiffDel:     lipgloss.Color("9"),
		ToolColor:   lipgloss.Color("14"),
		Highlight:   lipgloss.Color("15"),
	}
}

func lightScheme() ColorScheme {
	return ColorScheme{
		Name:        ThemeLight,
		Background:  lipgloss.Color("15"),
		Foreground:  lipgloss.Color("0"),
		Primary:     lipgloss.Color("4"),
		Secondary:   lipgloss.Color("6"),
		Accent:      lipgloss.Color("5"),
		Success:     lipgloss.Color("2"),
		Warning:     lipgloss.Color("3"),
		Error:       lipgloss.Color("1"),
		Dim:         lipgloss.Color("244"),
		Border:      lipgloss.Color("244"),
		UserColor:   lipgloss.Color("4"),
		AssistColor: lipgloss.Color("2"),
		SystemColor: lipgloss.Color("3"),
		CodeBg:      lipgloss.Color("254"),
		CodeFg:      lipgloss.Color("0"),
		DiffAdd:     lipgloss.Color("2"),
		DiffDel:     lipgloss.Color("1"),
		ToolColor:   lipgloss.Color("6"),
		Highlight:   lipgloss.Color("0"),
	}
}

func monokaiScheme() ColorScheme {
	return ColorScheme{
		Name:        ThemeMonokai,
		Background:  lipgloss.Color("#272822"),
		Foreground:  lipgloss.Color("#F8F8F2"),
		Primary:     lipgloss.Color("#66D9EF"),
		Secondary:   lipgloss.Color("#A6E22E"),
		Accent:      lipgloss.Color("#F92672"),
		Success:     lipgloss.Color("#A6E22E"),
		Warning:     lipgloss.Color("#E6DB74"),
		Error:       lipgloss.Color("#F92672"),
		Dim:         lipgloss.Color("#75715E"),
		Border:      lipgloss.Color("#75715E"),
		UserColor:   lipgloss.Color("#66D9EF"),
		AssistColor: lipgloss.Color("#A6E22E"),
		SystemColor: lipgloss.Color("#E6DB74"),
		CodeBg:      lipgloss.Color("#3E3D32"),
		CodeFg:      lipgloss.Color("#F8F8F2"),
		DiffAdd:     lipgloss.Color("#A6E22E"),
		DiffDel:     lipgloss.Color("#F92672"),
		ToolColor:   lipgloss.Color("#AE81FF"),
		Highlight:   lipgloss.Color("#F8F8F0"),
	}
}

func solarizedScheme() ColorScheme {
	return ColorScheme{
		Name:        ThemeSolarized,
		Background:  lipgloss.Color("#002B36"),
		Foreground:  lipgloss.Color("#839496"),
		Primary:     lipgloss.Color("#268BD2"),
		Secondary:   lipgloss.Color("#2AA198"),
		Accent:      lipgloss.Color("#D33682"),
		Success:     lipgloss.Color("#859900"),
		Warning:     lipgloss.Color("#B58900"),
		Error:       lipgloss.Color("#DC322F"),
		Dim:         lipgloss.Color("#586E75"),
		Border:      lipgloss.Color("#586E75"),
		UserColor:   lipgloss.Color("#268BD2"),
		AssistColor: lipgloss.Color("#859900"),
		SystemColor: lipgloss.Color("#B58900"),
		CodeBg:      lipgloss.Color("#073642"),
		CodeFg:      lipgloss.Color("#93A1A1"),
		DiffAdd:     lipgloss.Color("#859900"),
		DiffDel:     lipgloss.Color("#DC322F"),
		ToolColor:   lipgloss.Color("#2AA198"),
		Highlight:   lipgloss.Color("#FDF6E3"),
	}
}

func nordScheme() ColorScheme {
	return ColorScheme{
		Name:        ThemeNord,
		Background:  lipgloss.Color("#2E3440"),
		Foreground:  lipgloss.Color("#D8DEE9"),
		Primary:     lipgloss.Color("#81A1C1"),
		Secondary:   lipgloss.Color("#88C0D0"),
		Accent:      lipgloss.Color("#B48EAD"),
		Success:     lipgloss.Color("#A3BE8C"),
		Warning:     lipgloss.Color("#EBCB8B"),
		Error:       lipgloss.Color("#BF616A"),
		Dim:         lipgloss.Color("#4C566A"),
		Border:      lipgloss.Color("#4C566A"),
		UserColor:   lipgloss.Color("#81A1C1"),
		AssistColor: lipgloss.Color("#A3BE8C"),
		SystemColor: lipgloss.Color("#EBCB8B"),
		CodeBg:      lipgloss.Color("#3B4252"),
		CodeFg:      lipgloss.Color("#ECEFF4"),
		DiffAdd:     lipgloss.Color("#A3BE8C"),
		DiffDel:     lipgloss.Color("#BF616A"),
		ToolColor:   lipgloss.Color("#88C0D0"),
		Highlight:   lipgloss.Color("#ECEFF4"),
	}
}

// BuildStyles creates lipgloss styles from a color scheme.
func BuildStyles(cs ColorScheme) Styles {
	return Styles{
		User:      lipgloss.NewStyle().Foreground(cs.UserColor).Bold(true),
		Assistant: lipgloss.NewStyle().Foreground(cs.AssistColor).Bold(true),
		System:    lipgloss.NewStyle().Foreground(cs.SystemColor).Italic(true),
		Error:     lipgloss.NewStyle().Foreground(cs.Error).Bold(true),
		Dim:       lipgloss.NewStyle().Foreground(cs.Dim),
		Tool:      lipgloss.NewStyle().Foreground(cs.ToolColor).Italic(true),
		Code:      lipgloss.NewStyle().Foreground(cs.CodeFg),
		Highlight: lipgloss.NewStyle().Foreground(cs.Highlight).Bold(true),
		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cs.Border),
		StatusBar: lipgloss.NewStyle().
			Background(cs.CodeBg).
			Foreground(cs.Foreground).
			Padding(0, 1),
		DiffAdd: lipgloss.NewStyle().Foreground(cs.DiffAdd),
		DiffDel: lipgloss.NewStyle().Foreground(cs.DiffDel),
		Success: lipgloss.NewStyle().Foreground(cs.Success),
		Warning: lipgloss.NewStyle().Foreground(cs.Warning),
	}
}

// Styles holds pre-built lipgloss styles for a theme.
type Styles struct {
	User      lipgloss.Style
	Assistant lipgloss.Style
	System    lipgloss.Style
	Error     lipgloss.Style
	Dim       lipgloss.Style
	Tool      lipgloss.Style
	Code      lipgloss.Style
	Highlight lipgloss.Style
	Border    lipgloss.Style
	StatusBar lipgloss.Style
	DiffAdd   lipgloss.Style
	DiffDel   lipgloss.Style
	Success   lipgloss.Style
	Warning   lipgloss.Style
}
