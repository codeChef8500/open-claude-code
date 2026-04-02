package compact

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/provider"
)

const (
	// CompactThreshold is the fraction of maxTokens at which auto-compact triggers.
	CompactThreshold = 0.80

	// AutoCompactBufferTokens is subtracted from the effective context window
	// to determine the auto-compact threshold.
	AutoCompactBufferTokens = 13_000

	// WarningThresholdBufferTokens triggers a soft warning.
	WarningThresholdBufferTokens = 20_000

	// ErrorThresholdBufferTokens triggers an error-level warning.
	ErrorThresholdBufferTokens = 20_000

	// ManualCompactBufferTokens is reserved for manual /compact.
	ManualCompactBufferTokens = 3_000

	// MaxConsecutiveAutoCompactFailures is the circuit breaker limit.
	// After this many consecutive failures, auto-compact is skipped.
	MaxConsecutiveAutoCompactFailures = 3

	// MaxOutputTokensForSummary reserves tokens for compact output.
	MaxOutputTokensForSummary = 20_000

	// CompactMaxOutputTokens is the max tokens for the compact LLM call.
	CompactMaxOutputTokens = 16_384
)

// AutoCompactTrackingState tracks state across auto-compact iterations
// within a single query loop, aligned with claude-code-main.
type AutoCompactTrackingState struct {
	// Compacted is true after a successful compaction in this session.
	Compacted bool
	// TurnCounter counts turns since the last compaction.
	TurnCounter int
	// TurnID is a unique ID per turn (for analytics).
	TurnID string
	// ConsecutiveFailures tracks consecutive auto-compact failures.
	// Reset to 0 on success. Used as a circuit breaker.
	ConsecutiveFailures int
}

// RecompactionInfo provides diagnosis context for compaction analytics.
type RecompactionInfo struct {
	IsRecompactionInChain     bool
	TurnsSincePreviousCompact int
	PreviousCompactTurnID     string
	AutoCompactThreshold      int
	QuerySource               string
}

// TokenWarningState reports the context window usage state.
type TokenWarningState struct {
	PercentLeft                 int
	IsAboveWarningThreshold     bool
	IsAboveErrorThreshold       bool
	IsAboveAutoCompactThreshold bool
	IsAtBlockingLimit           bool
}

// GetEffectiveContextWindowSize returns the context window size minus
// reserved tokens for summary output.
func GetEffectiveContextWindowSize(contextWindow, maxOutputTokens int) int {
	reserved := maxOutputTokens
	if reserved > MaxOutputTokensForSummary {
		reserved = MaxOutputTokensForSummary
	}
	return contextWindow - reserved
}

// GetAutoCompactThreshold returns the token count at which auto-compact triggers.
func GetAutoCompactThreshold(contextWindow, maxOutputTokens int) int {
	effective := GetEffectiveContextWindowSize(contextWindow, maxOutputTokens)
	return effective - AutoCompactBufferTokens
}

// CalculateTokenWarningState computes the context window warning state.
func CalculateTokenWarningState(tokenUsage, contextWindow, maxOutputTokens int, autoCompactEnabled bool) TokenWarningState {
	autoThreshold := GetAutoCompactThreshold(contextWindow, maxOutputTokens)
	effective := GetEffectiveContextWindowSize(contextWindow, maxOutputTokens)

	threshold := effective
	if autoCompactEnabled {
		threshold = autoThreshold
	}

	percentLeft := 0
	if threshold > 0 {
		percentLeft = int(float64(threshold-tokenUsage) / float64(threshold) * 100)
		if percentLeft < 0 {
			percentLeft = 0
		}
	}

	warningThreshold := threshold - WarningThresholdBufferTokens
	errorThreshold := threshold - ErrorThresholdBufferTokens
	blockingLimit := effective - ManualCompactBufferTokens

	return TokenWarningState{
		PercentLeft:                 percentLeft,
		IsAboveWarningThreshold:     tokenUsage >= warningThreshold,
		IsAboveErrorThreshold:       tokenUsage >= errorThreshold,
		IsAboveAutoCompactThreshold: autoCompactEnabled && tokenUsage >= autoThreshold,
		IsAtBlockingLimit:           tokenUsage >= blockingLimit,
	}
}

// ShouldAutoCompact determines whether auto-compaction should trigger.
func ShouldAutoCompact(messages []*engine.Message, contextWindow, maxOutputTokens int) bool {
	tokenCount := estimateTokensFromMessages(messages)
	threshold := GetAutoCompactThreshold(contextWindow, maxOutputTokens)
	slog.Debug("autocompact: check",
		slog.Int("tokens", tokenCount),
		slog.Int("threshold", threshold))
	return tokenCount >= threshold
}

// AutoCompactResult holds the output of an LLM-driven compact operation.
type AutoCompactResult struct {
	Summary      string
	TokensBefore int
	TokensAfter  int
	// Usage holds the token usage from the compact LLM call itself.
	Usage *engine.UsageStats
}

// CompactionResult is the full result of a compaction operation, aligned
// with claude-code-main's CompactionResult interface.
type CompactionResult struct {
	// SummaryMessages are the synthetic user messages containing the summary.
	SummaryMessages []*engine.Message
	// MessagesToKeep are preserved recent messages (suffix-preserving compact).
	MessagesToKeep []*engine.Message
	// PreCompactTokenCount is the token count before compaction.
	PreCompactTokenCount int
	// PostCompactTokenCount is the estimated token count after compaction.
	PostCompactTokenCount int
	// TruePostCompactTokenCount is the actual token count after rebuild.
	TruePostCompactTokenCount int
	// CompactionUsage is the token usage from the compact LLM call.
	CompactionUsage *engine.UsageStats
	// UserDisplayMessage is an optional message from hooks to show the user.
	UserDisplayMessage string
}

// AutoCompactIfNeededResult is the return value of AutoCompactIfNeeded.
type AutoCompactIfNeededResult struct {
	WasCompacted        bool
	CompactionResult    *CompactionResult
	ConsecutiveFailures int
}

// AutoCompactIfNeeded checks whether auto-compaction should trigger and
// executes it if so, with circuit breaker logic.
func AutoCompactIfNeeded(
	ctx context.Context,
	prov provider.Provider,
	messages []*engine.Message,
	model string,
	contextWindow int,
	maxOutputTokens int,
	tracking *AutoCompactTrackingState,
) AutoCompactIfNeededResult {
	// Circuit breaker: stop after N consecutive failures.
	if tracking != nil && tracking.ConsecutiveFailures >= MaxConsecutiveAutoCompactFailures {
		slog.Debug("autocompact: circuit breaker tripped, skipping")
		return AutoCompactIfNeededResult{WasCompacted: false}
	}

	if !ShouldAutoCompact(messages, contextWindow, maxOutputTokens) {
		return AutoCompactIfNeededResult{WasCompacted: false}
	}

	result, err := RunAutoCompact(ctx, prov, messages, model, "")
	if err != nil {
		slog.Warn("autocompact: failed", slog.Any("err", err))
		prevFailures := 0
		if tracking != nil {
			prevFailures = tracking.ConsecutiveFailures
		}
		nextFailures := prevFailures + 1
		if nextFailures >= MaxConsecutiveAutoCompactFailures {
			slog.Warn("autocompact: circuit breaker tripped",
				slog.Int("failures", nextFailures))
		}
		return AutoCompactIfNeededResult{
			WasCompacted:        false,
			ConsecutiveFailures: nextFailures,
		}
	}

	summaryMsgs := SummaryToMessages(result.Summary)

	return AutoCompactIfNeededResult{
		WasCompacted: true,
		CompactionResult: &CompactionResult{
			SummaryMessages:       summaryMsgs,
			PreCompactTokenCount:  result.TokensBefore,
			PostCompactTokenCount: result.TokensAfter,
			CompactionUsage:       result.Usage,
		},
		ConsecutiveFailures: 0,
	}
}

// RunAutoCompact sends the conversation history to the LLM for summarisation
// and returns a compact result. Uses the detailed prompt from prompt.go.
func RunAutoCompact(
	ctx context.Context,
	prov provider.Provider,
	messages []*engine.Message,
	model string,
	customInstructions string,
) (*AutoCompactResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("auto compact: no messages to compact")
	}

	// Strip images from messages before sending for compaction.
	messages = StripImagesFromMessages(messages)

	// Build a plain-text transcript for the summariser.
	var sb strings.Builder
	for _, m := range messages {
		for _, b := range m.Content {
			if b.Type == engine.ContentTypeText && b.Text != "" {
				fmt.Fprintf(&sb, "[%s]: %s\n\n", m.Role, b.Text)
			}
		}
	}

	compactPrompt := GetCompactPrompt(customInstructions)

	params := provider.CallParams{
		Model:        model,
		MaxTokens:    CompactMaxOutputTokens,
		SystemPrompt: GetCompactSystemPrompt(),
		Messages: []*engine.Message{
			{
				Role: engine.RoleUser,
				Content: []*engine.ContentBlock{{
					Type: engine.ContentTypeText,
					Text: sb.String() + "\n\n" + compactPrompt,
				}},
			},
		},
		UsePromptCache: false,
	}

	eventCh, err := prov.CallModel(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("auto compact: %w", err)
	}

	var rawSummary strings.Builder
	var usage *engine.UsageStats
	for ev := range eventCh {
		switch ev.Type {
		case engine.EventTextDelta:
			rawSummary.WriteString(ev.Text)
		case engine.EventUsage:
			if ev.Usage != nil {
				usage = ev.Usage
			}
		}
	}

	formatted := FormatCompactSummary(rawSummary.String())
	if formatted == "" {
		return nil, fmt.Errorf("auto compact: empty summary from model")
	}

	return &AutoCompactResult{
		Summary:      formatted,
		TokensBefore: estimateTokens(sb.String()),
		TokensAfter:  estimateTokens(formatted),
		Usage:        usage,
	}, nil
}

// estimateTokens gives a rough character-based token estimate (1 token ≈ 4 chars).
func estimateTokens(text string) int {
	return len(text) / 4
}

// estimateTokensFromMessages estimates token count from a message slice.
func estimateTokensFromMessages(messages []*engine.Message) int {
	return estimateTokens(flattenText(messages))
}

// StripImagesFromMessages replaces image blocks with text markers before
// compaction, since images are not needed for generating summaries and
// can cause the compact call itself to hit prompt-too-long.
func StripImagesFromMessages(messages []*engine.Message) []*engine.Message {
	out := make([]*engine.Message, 0, len(messages))
	for _, m := range messages {
		hasImage := false
		for _, b := range m.Content {
			if b.Type == engine.ContentTypeImage {
				hasImage = true
				break
			}
		}
		if !hasImage {
			out = append(out, m)
			continue
		}
		// Replace image blocks with text markers.
		newBlocks := make([]*engine.ContentBlock, 0, len(m.Content))
		for _, b := range m.Content {
			if b.Type == engine.ContentTypeImage {
				newBlocks = append(newBlocks, &engine.ContentBlock{
					Type: engine.ContentTypeText,
					Text: "[image]",
				})
			} else {
				newBlocks = append(newBlocks, b)
			}
		}
		out = append(out, &engine.Message{
			ID:        m.ID,
			Role:      m.Role,
			Content:   newBlocks,
			Timestamp: m.Timestamp,
			SessionID: m.SessionID,
		})
	}
	return out
}
