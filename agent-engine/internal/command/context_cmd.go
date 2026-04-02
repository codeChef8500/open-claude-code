package command

import (
	"context"
	"fmt"
	"strings"
)

// ─── /context ─────────────────────────────────────────────────────────────────

// ContextCommand shows a summary of the current context window usage.
type ContextCommand struct{ BaseCommand }

func (c *ContextCommand) Name() string                  { return "context" }
func (c *ContextCommand) Description() string           { return "Show context window usage and token budget." }
func (c *ContextCommand) Type() CommandType             { return CommandTypeLocal }
func (c *ContextCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *ContextCommand) Execute(_ context.Context, _ []string, ectx *ExecContext) (string, error) {
	if ectx == nil {
		return "Context: no session active.", nil
	}
	// The context budget is stored in ExecContext if the engine wired it up.
	if ectx.ContextStats == nil {
		return "Context: token statistics not available (engine not wired).", nil
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
	return strings.Join(lines, "\n"), nil
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

func init() {
	defaultRegistry.Register(&ContextCommand{})
}
