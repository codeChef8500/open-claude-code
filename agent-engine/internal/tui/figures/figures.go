package figures

import "runtime"

// BlackCircle returns the platform-appropriate filled circle glyph.
// macOS uses ⏺ (U+23FA), others use ● (U+25CF).
func BlackCircle() string {
	if runtime.GOOS == "darwin" {
		return "⏺"
	}
	return "●"
}

const (
	// Message & response markers
	BulletOperator = "∙"       // U+2219
	ResponseIndent = "  ⎿  "   // assistant response connector
	Pointer        = "❯"       // U+276F prompt character
	BlockquoteBar  = "▎"       // U+258E left one-quarter block

	// Separators
	HeavyHorizontal = "━" // U+2501
	MiddleDot       = "·" // U+00B7

	// Status icons
	Tick    = "✓" // U+2713
	Cross   = "✗" // U+2717
	Warning = "⚠" // U+26A0
	Info    = "ℹ" // U+2139
	Circle  = "○" // U+25CB

	// Arrows
	ArrowUp   = "↑" // U+2191
	ArrowDown = "↓" // U+2193
	Lightning = "↯" // U+21AF

	// Effort levels
	EffortLow    = "○" // U+25CB
	EffortMedium = "◐" // U+25D0
	EffortHigh   = "●" // U+25CF
	EffortMax    = "◉" // U+25C9

	// Asterisks
	TeardropAsterisk = "✻" // U+273B

	// MCP/subscription indicators
	RefreshArrow  = "↻" // U+21BB
	ChannelArrow  = "←" // U+2190
	InjectedArrow = "→" // U+2192
	ForkGlyph     = "⑂" // U+2442

	// Review status
	DiamondOpen   = "◇" // U+25C7
	DiamondFilled = "◆" // U+25C6
	ReferenceMark = "※" // U+203B

	// Misc
	FlagIcon = "⚑" // U+2691
)

// SpinnerChars returns the platform-appropriate spinner animation characters.
// The sequence plays forward then reverses for a smooth oscillation.
func SpinnerChars() []string {
	if runtime.GOOS == "darwin" {
		return []string{"·", "✢", "✳", "✶", "✻", "✽"}
	}
	// Windows / Linux — ✳ may not render in all terminals
	return []string{"·", "✢", "*", "✶", "✻", "✽"}
}

// SpinnerFrames returns the full forward+reverse sequence for animation.
func SpinnerFrames() []string {
	chars := SpinnerChars()
	frames := make([]string, 0, len(chars)*2)
	frames = append(frames, chars...)
	for i := len(chars) - 1; i >= 0; i-- {
		frames = append(frames, chars[i])
	}
	return frames
}
