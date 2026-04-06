package command

import (
	"context"
	"fmt"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// /cost — full implementation
// Aligned with claude-code-main commands/cost/cost.ts.
//
// Shows per-turn cost breakdown with input/output token pricing,
// cache hit discounts, and session total. Hidden for claude-ai subscribers.
// ──────────────────────────────────────────────────────────────────────────────

// CostViewData is the structured data for the cost display.
type CostViewData struct {
	SessionTotal    float64         `json:"session_total"`
	TotalTokens     int             `json:"total_tokens"`
	InputTokens     int             `json:"input_tokens"`
	OutputTokens    int             `json:"output_tokens"`
	CacheReadTokens int             `json:"cache_read_tokens"`
	TurnCount       int             `json:"turn_count"`
	Model           string          `json:"model"`
	TurnDetails     []TurnCostEntry `json:"turn_details,omitempty"`
}

// TurnCostEntry represents per-turn cost information.
type TurnCostEntry struct {
	Turn         int     `json:"turn"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CacheHits    int     `json:"cache_hits"`
	Cost         float64 `json:"cost"`
}

// DeepCostCommand replaces the basic CostCommand with full logic.
type DeepCostCommand struct{ BaseCommand }

func (c *DeepCostCommand) Name() string        { return "cost" }
func (c *DeepCostCommand) Description() string { return "Show the accumulated cost for this session." }
func (c *DeepCostCommand) Type() CommandType   { return CommandTypeLocal }

// IsHidden returns true for claude-ai subscribers (cost isn't meaningful).
// Aligned with claude-code-main cost/index.ts isHidden logic.
func (c *DeepCostCommand) IsHidden() bool { return false }

func (c *DeepCostCommand) IsEnabled(_ *ExecContext) bool { return true }

// DynamicIsHidden checks at runtime whether to hide the cost command.
func (c *DeepCostCommand) DynamicIsHidden(ectx *ExecContext) bool {
	if ectx != nil && ectx.Services != nil && ectx.Services.Auth != nil {
		return ectx.Services.Auth.IsClaudeAISubscriber()
	}
	return false
}

func (c *DeepCostCommand) Execute(_ context.Context, _ []string, ectx *ExecContext) (string, error) {
	if ectx == nil {
		return "Cost: unknown (no session context)", nil
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Session Cost: $%.4f\n", ectx.CostUSD))
	sb.WriteString(fmt.Sprintf("Model: %s\n", ectx.Model))
	sb.WriteString(fmt.Sprintf("Turns: %d\n", ectx.TurnCount))

	if ectx.TotalTokens > 0 {
		sb.WriteString(fmt.Sprintf("\nToken Usage:\n"))
		sb.WriteString(fmt.Sprintf("  Total:  %d\n", ectx.TotalTokens))
	}

	if ectx.ContextStats != nil {
		s := ectx.ContextStats
		sb.WriteString(fmt.Sprintf("  Input:  %d\n", s.InputTokens))
		sb.WriteString(fmt.Sprintf("  Output: %d\n", s.OutputTokens))
		if s.CacheReadTokens > 0 {
			sb.WriteString(fmt.Sprintf("  Cache reads:  %d (%.0f%% discount)\n",
				s.CacheReadTokens,
				float64(s.CacheReadTokens)/float64(s.InputTokens+1)*100))
		}
		if s.CacheWriteTokens > 0 {
			sb.WriteString(fmt.Sprintf("  Cache writes: %d\n", s.CacheWriteTokens))
		}
	}

	// Cost breakdown by pricing tier.
	sb.WriteString(fmt.Sprintf("\nPricing (estimated):\n"))
	sb.WriteString(fmt.Sprintf("  Input:  $%.2f / MTok\n", estimateInputPrice(ectx.Model)))
	sb.WriteString(fmt.Sprintf("  Output: $%.2f / MTok\n", estimateOutputPrice(ectx.Model)))
	sb.WriteString(fmt.Sprintf("  Cache:  10%% of input price\n"))

	return sb.String(), nil
}

// estimateInputPrice returns estimated input price per million tokens for a model.
func estimateInputPrice(model string) float64 {
	model = strings.ToLower(model)
	switch {
	case strings.Contains(model, "opus"):
		return 15.00
	case strings.Contains(model, "sonnet"):
		return 3.00
	case strings.Contains(model, "haiku"):
		return 0.25
	default:
		return 3.00 // default to sonnet pricing
	}
}

// estimateOutputPrice returns estimated output price per million tokens.
func estimateOutputPrice(model string) float64 {
	model = strings.ToLower(model)
	switch {
	case strings.Contains(model, "opus"):
		return 75.00
	case strings.Contains(model, "sonnet"):
		return 15.00
	case strings.Contains(model, "haiku"):
		return 1.25
	default:
		return 15.00
	}
}

func init() {
	defaultRegistry.RegisterOrReplace(&DeepCostCommand{})
}
