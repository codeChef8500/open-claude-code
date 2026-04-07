package memory

import (
	"fmt"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// Memory age calculation — aligned with claude-code-main src/memdir/memoryAge.ts
// ────────────────────────────────────────────────────────────────────────────

// CalculateMemoryAge returns a human-readable age string for a modification time.
func CalculateMemoryAge(modTime time.Time) string {
	return CalculateMemoryAgeFrom(modTime, time.Now())
}

// CalculateMemoryAgeFrom returns a human-readable age string relative to a reference time.
func CalculateMemoryAgeFrom(modTime, now time.Time) string {
	d := now.Sub(modTime)
	if d < 0 {
		return "just now"
	}

	seconds := int(d.Seconds())
	minutes := int(d.Minutes())
	hours := int(d.Hours())
	days := hours / 24
	weeks := days / 7
	months := days / 30
	years := days / 365

	switch {
	case seconds < 60:
		return "just now"
	case minutes < 2:
		return "1 minute ago"
	case minutes < 60:
		return fmt.Sprintf("%d minutes ago", minutes)
	case hours < 2:
		return "1 hour ago"
	case hours < 24:
		return fmt.Sprintf("%d hours ago", hours)
	case days < 2:
		return "1 day ago"
	case days < 7:
		return fmt.Sprintf("%d days ago", days)
	case weeks < 2:
		return "1 week ago"
	case weeks < 5:
		return fmt.Sprintf("%d weeks ago", weeks)
	case months < 2:
		return "1 month ago"
	case months < 12:
		return fmt.Sprintf("%d months ago", months)
	case years < 2:
		return "1 year ago"
	default:
		return fmt.Sprintf("%d years ago", years)
	}
}

// IsStale reports whether a memory's modification time is older than the
// given threshold duration.
func IsStale(modTime time.Time, threshold time.Duration) bool {
	return time.Since(modTime) > threshold
}

// DefaultStalenessThreshold is the default duration after which a memory is
// considered stale (90 days).
const DefaultStalenessThreshold = 90 * 24 * time.Hour
