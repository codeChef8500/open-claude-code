package prompt

// TokenWarningLevel signals how close the context window is to its limit.
type TokenWarningLevel int

const (
	TokenWarningNone     TokenWarningLevel = 0
	TokenWarningApproach TokenWarningLevel = 1 // >85% used
	TokenWarningBlocking TokenWarningLevel = 2 // >95% used
)

const (
	warningThreshold  = 0.85
	blockingThreshold = 0.95
)

// GetTokenWarningLevel returns the appropriate warning level given current
// token usage vs. the context window limit.
func GetTokenWarningLevel(usedTokens, contextWindowSize int) TokenWarningLevel {
	if contextWindowSize <= 0 {
		return TokenWarningNone
	}
	ratio := float64(usedTokens) / float64(contextWindowSize)
	switch {
	case ratio >= blockingThreshold:
		return TokenWarningBlocking
	case ratio >= warningThreshold:
		return TokenWarningApproach
	default:
		return TokenWarningNone
	}
}

// TokenWarningMessage returns a human-readable warning to inject into the
// system prompt when approaching token limits.
func TokenWarningMessage(level TokenWarningLevel, used, limit int) string {
	switch level {
	case TokenWarningApproach:
		return "⚠️ Context window is getting full. Consider using /compact to summarise the conversation."
	case TokenWarningBlocking:
		return "🚫 Context window is nearly full. Please run /compact before continuing."
	default:
		return ""
	}
}
