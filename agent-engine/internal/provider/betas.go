package provider

// Beta feature header names recognised by the Anthropic API.
const (
	BetaPromptCaching  = "prompt-caching-2024-07-31"
	BetaThinking       = "interleaved-thinking-2025-05-14"
	BetaExtendedOutput = "output-128k-2025-02-19"
	BetaComputerUse    = "computer-use-2024-10-22"
	BetaFiles          = "files-api-2025-04-14"
)

// BetaSet is an ordered, deduplicated collection of beta header values.
type BetaSet struct {
	vals []string
	seen map[string]struct{}
}

// NewBetaSet creates an empty BetaSet.
func NewBetaSet() *BetaSet {
	return &BetaSet{seen: make(map[string]struct{})}
}

// Add adds one or more beta values, ignoring duplicates.
func (b *BetaSet) Add(vals ...string) {
	for _, v := range vals {
		if _, ok := b.seen[v]; !ok {
			b.seen[v] = struct{}{}
			b.vals = append(b.vals, v)
		}
	}
}

// Values returns the deduplicated beta header values in insertion order.
func (b *BetaSet) Values() []string {
	out := make([]string, len(b.vals))
	copy(out, b.vals)
	return out
}

// BuildBetas constructs the correct set of Anthropic beta headers for the
// given model and feature flags.
//
//   - useCache:    prompt caching is requested
//   - thinking:    extended thinking budget > 0
//   - extOutput:   extended output (128k) is requested
func BuildBetas(model string, useCache, thinking, extOutput bool) []string {
	spec := ResolveModel(model)
	b := NewBetaSet()

	for _, supported := range spec.SupportsBeta {
		switch supported {
		case BetaPromptCaching:
			if useCache {
				b.Add(BetaPromptCaching)
			}
		case BetaThinking:
			if thinking {
				b.Add(BetaThinking)
			}
		case BetaExtendedOutput:
			if extOutput {
				b.Add(BetaExtendedOutput)
			}
		}
	}
	return b.Values()
}
