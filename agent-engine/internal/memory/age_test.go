package memory

import (
	"testing"
	"time"
)

func TestCalculateMemoryAgeFrom(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		modTime time.Time
		want    string
	}{
		{"just now", now.Add(-10 * time.Second), "just now"},
		{"1 minute", now.Add(-90 * time.Second), "1 minute ago"},
		{"5 minutes", now.Add(-5 * time.Minute), "5 minutes ago"},
		{"1 hour", now.Add(-90 * time.Minute), "1 hour ago"},
		{"3 hours", now.Add(-3 * time.Hour), "3 hours ago"},
		{"1 day", now.Add(-36 * time.Hour), "1 day ago"},
		{"4 days", now.Add(-4 * 24 * time.Hour), "4 days ago"},
		{"1 week", now.Add(-10 * 24 * time.Hour), "1 week ago"},
		{"3 weeks", now.Add(-21 * 24 * time.Hour), "3 weeks ago"},
		{"1 month", now.Add(-45 * 24 * time.Hour), "1 month ago"},
		{"6 months", now.Add(-180 * 24 * time.Hour), "6 months ago"},
		{"1 year", now.Add(-400 * 24 * time.Hour), "1 year ago"},
		{"3 years", now.Add(-1100 * 24 * time.Hour), "3 years ago"},
		{"future", now.Add(1 * time.Hour), "just now"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateMemoryAgeFrom(tt.modTime, now)
			if got != tt.want {
				t.Errorf("CalculateMemoryAgeFrom() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsStale(t *testing.T) {
	fresh := time.Now().Add(-1 * time.Hour)
	stale := time.Now().Add(-100 * 24 * time.Hour)

	if IsStale(fresh, DefaultStalenessThreshold) {
		t.Error("1 hour old should not be stale")
	}
	if !IsStale(stale, DefaultStalenessThreshold) {
		t.Error("100 days old should be stale")
	}
}
