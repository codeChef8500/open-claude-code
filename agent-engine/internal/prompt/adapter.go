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

// Build assembles the 6-layer system prompt from the provided options.
func (a *Adapter) Build(opts engine.SystemPromptOptions) string {
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
	})
	return built.Text
}
