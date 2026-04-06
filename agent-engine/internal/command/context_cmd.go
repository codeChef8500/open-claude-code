package command

import (
	"context"
	"fmt"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// /context — Interactive context window visualization.
// Aligned with claude-code-main commands/context/context.tsx.
//
// Interactive mode: returns structured data for TUI grid visualization.
// Non-interactive mode: returns plain-text token statistics.
// ──────────────────────────────────────────────────────────────────────────────

// ContextCommand shows context window usage (interactive mode).
// The TUI renders a colorized grid where each block represents a token segment.
type ContextCommand struct{ BaseCommand }

func (c *ContextCommand) Name() string                  { return "context" }
func (c *ContextCommand) Aliases() []string             { return []string{"ctx"} }
func (c *ContextCommand) Description() string           { return "Show context window usage and token budget." }
func (c *ContextCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *ContextCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ContextCommand) ExecuteInteractive(_ context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := buildContextData(ectx)
	return &InteractiveResult{
		Component: "context",
		Data:      data,
	}, nil
}

// ContextNonInteractiveCommand is the plain-text fallback for non-interactive sessions.
// Aligned with claude-code-main commands/context/context-noninteractive.ts.
type ContextNonInteractiveCommand struct{ BaseCommand }

func (c *ContextNonInteractiveCommand) Name() string { return "context-text" }
func (c *ContextNonInteractiveCommand) Description() string {
	return "Show context window usage (text mode)."
}
func (c *ContextNonInteractiveCommand) Type() CommandType             { return CommandTypeLocal }
func (c *ContextNonInteractiveCommand) IsHidden() bool                { return true }
func (c *ContextNonInteractiveCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ContextNonInteractiveCommand) Execute(_ context.Context, _ []string, ectx *ExecContext) (string, error) {
	return formatContextText(ectx), nil
}

// ─── ContextStats ─────────────────────────────────────────────────────────────

// ContextStats carries token budget information for the /context command.
type ContextStats struct {
	InputTokens       int
	OutputTokens      int
	CacheReadTokens   int
	CacheWriteTokens  int
	ContextWindowSize int
	UsedFraction      float64
}

// ─── Token Category Breakdown ────────────────────────────────────────────────

// TokenCategory represents a category of tokens in the context window.
// Aligned with the colorized grid in claude-code-main context.tsx.
type TokenCategory struct {
	Name       string  `json:"name"`
	Tokens     int     `json:"tokens"`
	Percentage float64 `json:"percentage"`
	Color      string  `json:"color"` // TUI color for grid rendering
}

// ContextViewData is the structured data passed to the TUI component.
type ContextViewData struct {
	// Overall stats
	InputTokens       int     `json:"input_tokens"`
	OutputTokens      int     `json:"output_tokens"`
	CacheReadTokens   int     `json:"cache_read_tokens"`
	CacheWriteTokens  int     `json:"cache_write_tokens"`
	ContextWindowSize int     `json:"context_window_size"`
	UsedFraction      float64 `json:"used_fraction"`

	// Category breakdown for grid visualization
	Categories []TokenCategory `json:"categories"`

	// Grid data — each cell represents a token block
	GridCols   int      `json:"grid_cols"`
	GridBlocks []string `json:"grid_blocks"` // category name per block

	// Warning state
	NearlyFull bool `json:"nearly_full"`
}

// buildContextData constructs the full context visualization data.
func buildContextData(ectx *ExecContext) *ContextViewData {
	data := &ContextViewData{
		GridCols: 40, // 40 columns for the grid
	}

	if ectx == nil || ectx.ContextStats == nil {
		return data
	}

	s := ectx.ContextStats
	data.InputTokens = s.InputTokens
	data.OutputTokens = s.OutputTokens
	data.CacheReadTokens = s.CacheReadTokens
	data.CacheWriteTokens = s.CacheWriteTokens
	data.ContextWindowSize = s.ContextWindowSize
	data.UsedFraction = s.UsedFraction
	data.NearlyFull = s.UsedFraction >= 0.85

	// Estimate category breakdown from available data.
	// The engine can provide more detailed stats; these are estimates.
	totalUsed := s.InputTokens + s.OutputTokens
	if totalUsed == 0 {
		return data
	}

	// Estimate system prompt as ~15% of input, tool results ~40%, user ~20%, assistant ~25%
	sysTokens := int(float64(s.InputTokens) * 0.15)
	toolTokens := int(float64(s.InputTokens) * 0.40)
	userTokens := int(float64(s.InputTokens) * 0.20)
	asstTokens := s.InputTokens - sysTokens - toolTokens - userTokens + s.OutputTokens

	data.Categories = []TokenCategory{
		{Name: "System Prompt", Tokens: sysTokens, Percentage: pct(sysTokens, totalUsed), Color: "blue"},
		{Name: "Tool Results", Tokens: toolTokens, Percentage: pct(toolTokens, totalUsed), Color: "green"},
		{Name: "User Messages", Tokens: userTokens, Percentage: pct(userTokens, totalUsed), Color: "yellow"},
		{Name: "Assistant", Tokens: asstTokens, Percentage: pct(asstTokens, totalUsed), Color: "magenta"},
	}

	// Build grid blocks for visualization.
	totalBlocks := data.GridCols * 5 // 5 rows of 40 columns = 200 blocks
	usedBlocks := int(s.UsedFraction * float64(totalBlocks))
	if usedBlocks > totalBlocks {
		usedBlocks = totalBlocks
	}

	data.GridBlocks = make([]string, totalBlocks)
	blockIdx := 0
	for _, cat := range data.Categories {
		catBlocks := int(cat.Percentage / 100.0 * float64(usedBlocks))
		for i := 0; i < catBlocks && blockIdx < totalBlocks; i++ {
			data.GridBlocks[blockIdx] = cat.Name
			blockIdx++
		}
	}
	// Fill remaining used blocks with last category.
	for blockIdx < usedBlocks && blockIdx < totalBlocks {
		data.GridBlocks[blockIdx] = "Assistant"
		blockIdx++
	}
	// Empty blocks.
	for blockIdx < totalBlocks {
		data.GridBlocks[blockIdx] = ""
		blockIdx++
	}

	return data
}

// formatContextText formats context stats as plain text (non-interactive mode).
func formatContextText(ectx *ExecContext) string {
	if ectx == nil {
		return "Context: no session active."
	}
	if ectx.ContextStats == nil {
		return "Context: token statistics not available (engine not wired)."
	}
	s := ectx.ContextStats
	bar := buildProgressBar(s.UsedFraction, 30)
	lines := []string{
		fmt.Sprintf("Context Window: %s %.0f%%", bar, s.UsedFraction*100),
		fmt.Sprintf("  Input tokens:  %d / %d", s.InputTokens, s.ContextWindowSize),
		fmt.Sprintf("  Output tokens: %d", s.OutputTokens),
		fmt.Sprintf("  Cache reads:   %d tokens", s.CacheReadTokens),
		fmt.Sprintf("  Cache writes:  %d tokens", s.CacheWriteTokens),
	}
	if s.UsedFraction >= 0.85 {
		lines = append(lines, "  ⚠ Context window is nearly full. Consider /compact.")
	}
	return strings.Join(lines, "\n")
}

// buildProgressBar renders a simple ASCII progress bar of width w.
func buildProgressBar(fraction float64, w int) string {
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	filled := int(fraction * float64(w))
	bar := "[" + strings.Repeat("█", filled) + strings.Repeat("░", w-filled) + "]"
	return bar
}

// pct computes percentage safely.
func pct(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

func init() {
	defaultRegistry.Register(
		&ContextCommand{},
		&ContextNonInteractiveCommand{},
	)
}
