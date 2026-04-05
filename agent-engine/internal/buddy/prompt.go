package buddy

import "fmt"

// CompanionIntroText generates the system prompt snippet that informs the main
// LLM about the companion's presence, matching claude-code-main prompt.ts.
func CompanionIntroText(name string, species string) string {
	return fmt.Sprintf(`# Companion

A small %s named %s sits beside the user's input box and occasionally comments in a speech bubble. You're not %s — it's a separate watcher.

When the user addresses %s directly (by name), its bubble will answer. Your job in that moment is to stay out of the way: respond in ONE line or less, or just answer any part of the message meant for you. Don't explain that you're not %s — they know. Don't narrate what %s might say — the bubble handles that.`, species, name, name, name, name, name)
}

// ShouldInjectIntro checks if a companion intro should be injected.
// Returns the intro text and true if injection is needed, empty and false otherwise.
// Dedup: if the companion name already appears in an existing intro, skip.
func ShouldInjectIntro(comp *Companion, existingIntros []string) (string, bool) {
	if comp == nil || comp.Name == "" {
		return "", false
	}

	// Check if already announced
	for _, intro := range existingIntros {
		if intro == comp.Name {
			return "", false
		}
	}

	return CompanionIntroText(comp.Name, string(comp.Species)), true
}
