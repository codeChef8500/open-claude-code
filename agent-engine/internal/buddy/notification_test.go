package buddy

import (
	"testing"
)

func TestIsBuddyLive(t *testing.T) {
	// This is a time-dependent test. As of April 2026+, should be true.
	// We just verify it doesn't panic and returns a bool.
	_ = IsBuddyLive()
}

func TestIsBuddyTeaserWindow(t *testing.T) {
	_ = IsBuddyTeaserWindow()
}

func TestTeaserNotification_HasCompanion(t *testing.T) {
	// If user already has a companion, teaser should be empty.
	result := TeaserNotification(true)
	if result != "" {
		t.Errorf("expected empty teaser for existing companion, got %q", result)
	}
}
