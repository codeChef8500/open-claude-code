package provider

import "strings"

// ModelFamily classifies a model string into a broad family.
type ModelFamily string

const (
	ModelFamilyClaude  ModelFamily = "claude"
	ModelFamilyGPT     ModelFamily = "gpt"
	ModelFamilyGemini  ModelFamily = "gemini"
	ModelFamilyUnknown ModelFamily = "unknown"
)

// ModelSpec holds resolved properties for a given model name.
type ModelSpec struct {
	Name           string
	Family         ModelFamily
	ContextWindow  int
	MaxOutputTokens int
	SupportsThinking bool
	SupportsBeta   []string // beta features this model supports
}

// wellKnownModels maps canonical model name prefixes to their specs.
// These are best-effort defaults; callers may override via config.
var wellKnownModels = []ModelSpec{
	{
		Name:             "claude-opus-4",
		Family:           ModelFamilyClaude,
		ContextWindow:    200_000,
		MaxOutputTokens:  32_000,
		SupportsThinking: true,
		SupportsBeta:     []string{BetaThinking, BetaPromptCaching, BetaExtendedOutput},
	},
	{
		Name:             "claude-sonnet-4",
		Family:           ModelFamilyClaude,
		ContextWindow:    200_000,
		MaxOutputTokens:  16_000,
		SupportsThinking: true,
		SupportsBeta:     []string{BetaThinking, BetaPromptCaching, BetaExtendedOutput},
	},
	{
		Name:             "claude-haiku-3-5",
		Family:           ModelFamilyClaude,
		ContextWindow:    200_000,
		MaxOutputTokens:  8_192,
		SupportsThinking: false,
		SupportsBeta:     []string{BetaPromptCaching},
	},
	{
		Name:             "claude-3-5-sonnet",
		Family:           ModelFamilyClaude,
		ContextWindow:    200_000,
		MaxOutputTokens:  8_192,
		SupportsThinking: false,
		SupportsBeta:     []string{BetaPromptCaching},
	},
	{
		Name:             "claude-3-5-haiku",
		Family:           ModelFamilyClaude,
		ContextWindow:    200_000,
		MaxOutputTokens:  8_192,
		SupportsThinking: false,
		SupportsBeta:     []string{BetaPromptCaching},
	},
	{
		Name:             "claude-3-opus",
		Family:           ModelFamilyClaude,
		ContextWindow:    200_000,
		MaxOutputTokens:  4_096,
		SupportsThinking: false,
		SupportsBeta:     []string{BetaPromptCaching},
	},
	{
		Name:             "gpt-4o",
		Family:           ModelFamilyGPT,
		ContextWindow:    128_000,
		MaxOutputTokens:  4_096,
		SupportsThinking: false,
	},
	{
		Name:             "gpt-4-turbo",
		Family:           ModelFamilyGPT,
		ContextWindow:    128_000,
		MaxOutputTokens:  4_096,
		SupportsThinking: false,
	},
}

// ResolveModel returns the ModelSpec for a given model name string.
// It performs prefix matching so "claude-sonnet-4-20250514" matches "claude-sonnet-4".
// Falls back to a best-guess spec based on the name prefix.
func ResolveModel(name string) ModelSpec {
	lower := strings.ToLower(name)
	for _, spec := range wellKnownModels {
		if strings.HasPrefix(lower, spec.Name) {
			spec.Name = name // preserve the caller's exact name
			return spec
		}
	}
	// Heuristic fallbacks.
	spec := ModelSpec{Name: name, ContextWindow: 200_000, MaxOutputTokens: 8_192}
	switch {
	case strings.HasPrefix(lower, "claude"):
		spec.Family = ModelFamilyClaude
		spec.SupportsBeta = []string{BetaPromptCaching}
	case strings.HasPrefix(lower, "gpt"):
		spec.Family = ModelFamilyGPT
	case strings.HasPrefix(lower, "gemini"):
		spec.Family = ModelFamilyGemini
	default:
		spec.Family = ModelFamilyUnknown
	}
	return spec
}

// IsClaude reports whether the model name refers to a Claude model.
func IsClaude(model string) bool {
	return strings.HasPrefix(strings.ToLower(model), "claude")
}

// IsThinkingModel reports whether the given model supports extended thinking.
func IsThinkingModel(model string) bool {
	return ResolveModel(model).SupportsThinking
}

// ContextWindowFor returns the context window size for the given model.
func ContextWindowFor(model string) int {
	if n := ResolveModel(model).ContextWindow; n > 0 {
		return n
	}
	return 200_000 // safe default
}
