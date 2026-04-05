package prompt

import (
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// Adapter implements engine.SystemPromptBuilder backed by BuildEffectiveSystemPrompt.
// It is wired at SDK construction time to avoid an import cycle between the
// engine and prompt packages.
type Adapter struct{}

// NewAdapter creates a prompt.Adapter.
func NewAdapter() *Adapter { return &Adapter{} }

// BuildParts assembles the 6-layer system prompt and returns both the flat text
// and the ordered cache-aware segments.
func (a *Adapter) BuildParts(opts engine.SystemPromptOptions) engine.SystemPromptResult {
	// Convert engine.Tool slice to tool.Tool slice (same underlying type via alias).
	tools := make([]tool.Tool, len(opts.Tools))
	for i, t := range opts.Tools {
		tools[i] = t
	}

	var uctx *tool.UseContext
	if opts.UseContext != nil {
		uctx = &tool.UseContext{
			WorkDir:   opts.UseContext.WorkDir,
			SessionID: opts.UseContext.SessionID,
			AutoMode:  opts.UseContext.AutoMode,
		}
	}

	built := BuildEffectiveSystemPrompt(BuildOptions{
		Tools:              tools,
		UseContext:         uctx,
		WorkDir:            opts.WorkDir,
		MemoryContent:      opts.MemoryContent,
		CustomSystemPrompt: opts.CustomSystemPrompt,
		AppendSystemPrompt: opts.AppendSystemPrompt,
		KairosActive:       opts.KairosActive,
		BuddyActive:        opts.BuddyActive,
		CompanionName:      opts.CompanionName,
		CompanionSpecies:   opts.CompanionSpecies,
	})

	// Map prompt parts to engine.SystemPromptPart slices.
	parts := make([]engine.SystemPromptPart, 0, len(built.Parts))
	for _, p := range built.Parts {
		parts = append(parts, engine.SystemPromptPart{
			Content:   p.Content,
			CacheHint: p.CacheHint,
		})
	}

	return engine.SystemPromptResult{
		Text:  built.Text,
		Parts: parts,
	}
}
