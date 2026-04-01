package mode

import (
	"os"
)

const (
	fastModeModel      = "claude-haiku-4-5"
	fastModeEnvKey     = "AGENT_ENGINE_FAST_MODE"
	fastModeUnavailMsg = "Fast mode requires a Haiku-class model to be configured."
)

// IsFastModeAvailable reports whether fast mode can be activated given the
// current configuration.
func IsFastModeAvailable(configuredModel string) bool {
	// Fast mode is available if explicitly enabled via env or if a Haiku
	// model is already configured.
	if IsEnvTruthy(os.Getenv(fastModeEnvKey)) {
		return true
	}
	return isFastModel(configuredModel)
}

// GetFastModeUnavailableReason returns a human-readable explanation for why
// fast mode cannot be activated, or "" if it is available.
func GetFastModeUnavailableReason(configuredModel string) string {
	if IsFastModeAvailable(configuredModel) {
		return ""
	}
	return fastModeUnavailMsg
}

// FastModeModel returns the model name to use in fast mode.
func FastModeModel() string { return fastModeModel }

func isFastModel(model string) bool {
	fastModels := []string{"haiku", "flash", "mini", "nano"}
	for _, fm := range fastModels {
		if containsIgnoreCase(model, fm) {
			return true
		}
	}
	return false
}

func containsIgnoreCase(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	ls := toLower(s)
	lsub := toLower(sub)
	return contains(ls, lsub)
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
