package buddy

import "time"

// ─── Notification ────────────────────────────────────────────────────────────
// Buddy teaser notification and feature-flag checks.
// Matches claude-code-main useBuddyNotification.tsx logic.

// IsBuddyLive returns true if the buddy feature should be active.
// Always live after April 2026 (matching claude-code-main).
func IsBuddyLive() bool {
	d := time.Now()
	y := d.Year()
	m := d.Month()
	return y > 2026 || (y == 2026 && m >= time.April)
}

// IsBuddyTeaserWindow returns true during the teaser window (April 1-7, 2026).
func IsBuddyTeaserWindow() bool {
	d := time.Now()
	return d.Year() == 2026 && d.Month() == time.April && d.Day() <= 7
}

// TeaserNotification returns the teaser text if we're in the teaser window
// and the user has no companion yet. Returns empty string otherwise.
func TeaserNotification(hasCompanion bool) string {
	if hasCompanion || !IsBuddyTeaserWindow() {
		return ""
	}
	return "/buddy"
}

// ColoredChar holds a single character and its ANSI-compatible hex color.
type ColoredChar struct {
	Char  rune
	Color string // hex color, e.g. "#ff0000"
}

// RainbowColors are the 7 rainbow colors used for the teaser notification,
// matching claude-code-main RainbowText component.
var RainbowColors = []string{
	"#ff0000", // red
	"#ff8800", // orange
	"#ffff00", // yellow
	"#00ff00", // green
	"#0088ff", // blue
	"#8800ff", // indigo
	"#ff00ff", // violet
}

// TeaserRainbowParts returns each character of "/buddy" with a rainbow color.
// Returns nil if not in the teaser window or user already has a companion.
func TeaserRainbowParts(hasCompanion bool) []ColoredChar {
	text := TeaserNotification(hasCompanion)
	if text == "" {
		return nil
	}
	runes := []rune(text)
	parts := make([]ColoredChar, len(runes))
	for i, r := range runes {
		parts[i] = ColoredChar{
			Char:  r,
			Color: RainbowColors[i%len(RainbowColors)],
		}
	}
	return parts
}
