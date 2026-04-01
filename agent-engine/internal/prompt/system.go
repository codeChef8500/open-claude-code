package prompt

import (
	"strings"

	"github.com/wall-ai/agent-engine/internal/tool"
)

// BuildOptions holds all inputs needed to construct the full system prompt.
type BuildOptions struct {
	Tools              []tool.Tool
	UseContext         *tool.UseContext
	WorkDir            string
	CustomSystemPrompt string
	AppendSystemPrompt string
	MemoryContent      string // pre-fetched CLAUDE.md merged content
	SkipCacheWrite     bool
}

// BuiltSystemPrompt is the result of BuildEffectiveSystemPrompt.
type BuiltSystemPrompt struct {
	// Full combined text (for non-cache-aware providers)
	Text string
	// Ordered parts for cache-aware injection (Anthropic multi-block)
	Parts []PromptPart
}

// BuildEffectiveSystemPrompt assembles the 6-layer system prompt in
// cache-friendly order (stable → dynamic).
//
//	Layer 1 – base_prompt (go:embed, most stable)
//	Layer 2 – tool descriptions (changes only when tools change)
//	Layer 3 – memory content (CLAUDE.md; changes per project)
//	Layer 4 – environment context (platform, cwd, time — dynamic)
//	Layer 5 – custom system prompt (user-supplied override)
//	Layer 6 – append system prompt (appended without affecting cache layers 1-5)
func BuildEffectiveSystemPrompt(opts BuildOptions) *BuiltSystemPrompt {
	var parts []PromptPart

	// Layer 1 – base prompt (most cache-stable)
	base := GetBasePrompt()
	if base != "" {
		parts = append(parts, PromptPart{
			Content:   base,
			Order:     CacheOrderBasePrompt,
			CacheHint: !opts.SkipCacheWrite,
		})
	}

	// Layer 2 – tool descriptions
	if len(opts.Tools) > 0 && opts.UseContext != nil {
		toolDesc := BuildToolsPrompt(opts.Tools, opts.UseContext)
		if toolDesc != "" {
			parts = append(parts, PromptPart{
				Content: toolDesc,
				Order:   CacheOrderToolDescs,
			})
		}
	}

	// Layer 3 – memory content
	if opts.MemoryContent != "" {
		parts = append(parts, PromptPart{
			Content: opts.MemoryContent,
			Order:   CacheOrderMemories,
		})
	}

	// Layer 4 – environment context (dynamic; do not cache)
	envCtx := BuildEnvContext(opts.WorkDir)
	parts = append(parts, PromptPart{
		Content: envCtx.Render(),
		Order:   CacheOrderEnvironment,
	})

	// Layer 5 – custom system prompt
	if opts.CustomSystemPrompt != "" {
		parts = append(parts, PromptPart{
			Content: opts.CustomSystemPrompt,
			Order:   CacheOrderCustomPrompt,
		})
	}

	// Sort parts into cache-friendly order.
	parts = SortParts(parts)

	// Build combined text.
	var texts []string
	for _, p := range parts {
		if p.Content != "" {
			texts = append(texts, p.Content)
		}
	}

	combined := strings.Join(texts, "\n\n")

	// Layer 6 – append (after join, not in parts so it doesn't affect cache)
	if opts.AppendSystemPrompt != "" {
		combined += "\n\n" + opts.AppendSystemPrompt
	}

	return &BuiltSystemPrompt{
		Text:  combined,
		Parts: parts,
	}
}
