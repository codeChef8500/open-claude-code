package util

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Truncate shortens text to at most maxLength runes. If truncated, it keeps the
// first and last portions and inserts "..." in the middle.
func Truncate(text string, maxLength int) string {
	runes := []rune(text)
	if len(runes) <= maxLength {
		return text
	}
	if maxLength <= 3 {
		return "..."
	}
	half := (maxLength - 3) / 2
	return string(runes[:half]) + "..." + string(runes[len(runes)-half:])
}

// FormatFileSize formats bytes as a human-readable string (B / KB / MB / GB).
func FormatFileSize(bytes int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FormatCost formats a USD amount as a human-readable cost string.
// Values below $0.001 are shown in sub-cent notation.
func FormatCost(usd float64) string {
	if usd == 0 {
		return "$0.00"
	}
	if usd < 0.001 {
		return fmt.Sprintf("$%.6f", usd)
	}
	if usd < 0.01 {
		return fmt.Sprintf("$%.4f", usd)
	}
	return fmt.Sprintf("$%.2f", usd)
}

// FormatRelativeDate returns a human-friendly relative date string
// (e.g. "just now", "5 minutes ago", "2 hours ago", "3 days ago").
func FormatRelativeDate(t time.Time) string {
	diff := time.Since(t)
	if diff < 0 {
		diff = 0
	}
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		return fmt.Sprintf("%d minute%s ago", mins, plural(mins))
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		return fmt.Sprintf("%d hour%s ago", hours, plural(hours))
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d day%s ago", days, plural(days))
	default:
		return t.Format("2006-01-02")
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// PadRight pads s with spaces on the right to reach width.
func PadRight(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(runes))
}

// RoundToDecimals rounds f to n decimal places.
func RoundToDecimals(f float64, n int) float64 {
	p := math.Pow10(n)
	return math.Round(f*p) / p
}
