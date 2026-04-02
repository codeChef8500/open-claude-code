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
	Name                  string
	Family                ModelFamily
	ContextWindow         int
	MaxOutputTokens       int
	SupportsThinking      bool
	SupportsBeta          []string // beta features this model supports
	SupportsWebSearch     bool
	SupportsFastMode      bool
	DefaultThinkingBudget int // 0 = no default
}

// wellKnownModels maps canonical model name prefixes to their specs.
// These are best-effort defaults; callers may override via config.
var wellKnownModels = []ModelSpec{
	// ── Opus family ────────────────────────────────────────────────────
	{
		Name:                  "claude-opus-4-6",
		Family:                ModelFamilyClaude,
		ContextWindow:         200_000,
		MaxOutputTokens:       32_000,
		SupportsThinking:      true,
		SupportsFastMode:      true,
		SupportsWebSearch:     true,
		DefaultThinkingBudget: 10_000,
		SupportsBeta:          []string{BetaThinking, BetaPromptCaching, BetaExtendedOutput},
	},
	{
		Name:                  "claude-opus-4-5",
		Family:                ModelFamilyClaude,
		ContextWindow:         200_000,
		MaxOutputTokens:       32_000,
		SupportsThinking:      true,
		SupportsWebSearch:     true,
		DefaultThinkingBudget: 10_000,
		SupportsBeta:          []string{BetaThinking, BetaPromptCaching, BetaExtendedOutput},
	},
	{
		Name:                  "claude-opus-4-1",
		Family:                ModelFamilyClaude,
		ContextWindow:         200_000,
		MaxOutputTokens:       32_000,
		SupportsThinking:      true,
		SupportsWebSearch:     true,
		DefaultThinkingBudget: 10_000,
		SupportsBeta:          []string{BetaThinking, BetaPromptCaching, BetaExtendedOutput},
	},
	{
		Name:                  "claude-opus-4",
		Family:                ModelFamilyClaude,
		ContextWindow:         200_000,
		MaxOutputTokens:       32_000,
		SupportsThinking:      true,
		SupportsWebSearch:     true,
		DefaultThinkingBudget: 10_000,
		SupportsBeta:          []string{BetaThinking, BetaPromptCaching, BetaExtendedOutput},
	},
	// ── Sonnet family ──────────────────────────────────────────────────
	{
		Name:                  "claude-sonnet-4-6",
		Family:                ModelFamilyClaude,
		ContextWindow:         200_000,
		MaxOutputTokens:       16_000,
		SupportsThinking:      true,
		SupportsWebSearch:     true,
		DefaultThinkingBudget: 10_000,
		SupportsBeta:          []string{BetaThinking, BetaPromptCaching, BetaExtendedOutput},
	},
	{
		Name:                  "claude-sonnet-4-5",
		Family:                ModelFamilyClaude,
		ContextWindow:         200_000,
		MaxOutputTokens:       16_000,
		SupportsThinking:      true,
		SupportsWebSearch:     true,
		DefaultThinkingBudget: 10_000,
		SupportsBeta:          []string{BetaThinking, BetaPromptCaching, BetaExtendedOutput},
	},
	{
		Name:                  "claude-sonnet-4",
		Family:                ModelFamilyClaude,
		ContextWindow:         200_000,
		MaxOutputTokens:       16_000,
		SupportsThinking:      true,
		SupportsWebSearch:     true,
		DefaultThinkingBudget: 10_000,
		SupportsBeta:          []string{BetaThinking, BetaPromptCaching, BetaExtendedOutput},
	},
	{
		Name:             "claude-3-7-sonnet",
		Family:           ModelFamilyClaude,
		ContextWindow:    200_000,
		MaxOutputTokens:  8_192,
		SupportsThinking: true,
		SupportsBeta:     []string{BetaThinking, BetaPromptCaching},
	},
	{
		Name:             "claude-3-5-sonnet",
		Family:           ModelFamilyClaude,
		ContextWindow:    200_000,
		MaxOutputTokens:  8_192,
		SupportsThinking: false,
		SupportsBeta:     []string{BetaPromptCaching},
	},
	// ── Haiku family ───────────────────────────────────────────────────
	{
		Name:             "claude-haiku-4-5",
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
	// ── Non-Claude ─────────────────────────────────────────────────────
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
