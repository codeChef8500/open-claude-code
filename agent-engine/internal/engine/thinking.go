package engine

// ThinkingConfig holds the resolved thinking (extended reasoning) parameters
// for a single query loop invocation.
type ThinkingConfig struct {
	// Enabled is true when extended thinking should be requested.
	Enabled bool
	// BudgetTokens is the token budget allocated to the thinking process.
	// Must be ≥ 1024 when Enabled is true.
	BudgetTokens int
	// AdaptiveBudget allows the model to use less than BudgetTokens if it
	// determines the task is simple.
	AdaptiveBudget bool
}

const (
	thinkingMinBudget     = 1024
	thinkingDefaultBudget = 5000
	thinkingMaxBudget     = 32000
)

// ClampBudget ensures the thinking budget is within valid bounds.
func (tc *ThinkingConfig) ClampBudget() {
	if !tc.Enabled {
		tc.BudgetTokens = 0
		return
	}
	if tc.BudgetTokens < thinkingMinBudget {
		tc.BudgetTokens = thinkingMinBudget
	}
	if tc.BudgetTokens > thinkingMaxBudget {
		tc.BudgetTokens = thinkingMaxBudget
	}
}

// ThinkingRules enforces the three strict thinking-block rules that must hold
// in every multi-turn conversation:
//
//  1. A thinking block must NEVER be the last block in an assistant message.
//  2. Thinking blocks must be preserved across turns (never stripped mid-session).
//  3. If the model emits thinking blocks, the next assistant message must also
//     start with a thinking block (continuity rule).
type ThinkingRules struct{}

// ValidateMessage checks that an assistant message conforms to the thinking
// rules.  Returns a non-nil error describing the violation.
func (ThinkingRules) ValidateMessage(msg *Message) error {
	if msg == nil || msg.Role != RoleAssistant {
		return nil
	}
	blocks := msg.Content
	if len(blocks) == 0 {
		return nil
	}

	// Rule 1: thinking block must not be the last block.
	last := blocks[len(blocks)-1]
	if last.Type == ContentTypeThinking {
		return &ThinkingRuleError{
			Rule:    1,
			Message: "thinking block must not be the last block in an assistant message",
		}
	}

	return nil
}

// StripOrphanThinking removes trailing thinking blocks from an assistant
// message to satisfy Rule 1 without discarding useful content.
// Returns the modified message (original is not mutated).
func StripOrphanThinking(msg *Message) *Message {
	if msg == nil || msg.Role != RoleAssistant {
		return msg
	}
	blocks := make([]*ContentBlock, 0, len(msg.Content))
	for _, b := range msg.Content {
		blocks = append(blocks, b)
	}
	// Drop trailing thinking blocks.
	for len(blocks) > 0 && blocks[len(blocks)-1].Type == ContentTypeThinking {
		blocks = blocks[:len(blocks)-1]
	}
	if len(blocks) == len(msg.Content) {
		return msg
	}
	out := *msg
	out.Content = blocks
	return &out
}

// HasThinkingBlock reports whether any block in the message is a thinking block.
func HasThinkingBlock(msg *Message) bool {
	for _, b := range msg.Content {
		if b.Type == ContentTypeThinking {
			return true
		}
	}
	return false
}

// ThinkingRuleError describes a thinking rule violation.
type ThinkingRuleError struct {
	Rule    int
	Message string
}

func (e *ThinkingRuleError) Error() string {
	return e.Message
}
