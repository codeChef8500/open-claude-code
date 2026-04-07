package compact

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/memory"
	"github.com/wall-ai/agent-engine/internal/provider"
)

// ────────────────────────────────────────────────────────────────────────────
// Session-memory-aware compaction (SM-Compact) — aligned with claude-code-main
// ────────────────────────────────────────────────────────────────────────────

const (
	// SMCompactMinTurns is the minimum number of turns before session memory
	// extraction is considered.
	SMCompactMinTurns = 3
	// SMCompactMinToolCalls is the minimum number of tool calls in the session
	// before session memory extraction triggers.
	SMCompactMinToolCalls = 2
	// SMCompactRecentMessagesToKeep is the number of recent messages preserved
	// after compaction to maintain continuity.
	SMCompactRecentMessagesToKeep = 6
)

// SessionMemoryManager manages session memory extraction and storage,
// integrated with the compaction pipeline.
type SessionMemoryManager struct {
	prov        provider.Provider
	model       string
	projectRoot string
	// extractedFacts are the accumulated durable facts from this session.
	extractedFacts []string
	// turnCount tracks the number of assistant turns.
	turnCount int
	// toolCallCount tracks tool calls made in this session.
	toolCallCount int
	// lastExtractTime is the timestamp of the last extraction.
	lastExtractTime time.Time
	// enabled controls whether extraction runs.
	enabled bool
}

// NewSessionMemoryManager creates a new manager.
func NewSessionMemoryManager(prov provider.Provider, model, projectRoot string) *SessionMemoryManager {
	return &SessionMemoryManager{
		prov:        prov,
		model:       model,
		projectRoot: projectRoot,
		enabled:     memory.IsAutoMemoryEnabled(),
	}
}

// RecordTurn increments the turn counter. Call after each assistant response.
func (m *SessionMemoryManager) RecordTurn() {
	m.turnCount++
}

// RecordToolCall increments the tool call counter.
func (m *SessionMemoryManager) RecordToolCall() {
	m.toolCallCount++
}

// ShouldExtract reports whether conditions are met to extract session memories.
func (m *SessionMemoryManager) ShouldExtract() bool {
	if !m.enabled {
		return false
	}
	if m.turnCount < SMCompactMinTurns {
		return false
	}
	if m.toolCallCount < SMCompactMinToolCalls {
		return false
	}
	// Throttle: at most once per 5 minutes
	if !m.lastExtractTime.IsZero() && time.Since(m.lastExtractTime) < 5*time.Minute {
		return false
	}
	return true
}

// ExtractAndSave extracts session memories from the conversation and saves
// them to the auto-memory daily log.
func (m *SessionMemoryManager) ExtractAndSave(ctx context.Context, messages []*engine.Message) error {
	if !m.enabled || m.prov == nil {
		return nil
	}

	sm, err := ExtractSessionMemory(ctx, m.prov, messages, m.model)
	if err != nil {
		return fmt.Errorf("session memory extract: %w", err)
	}

	if len(sm.Facts) == 0 {
		return nil
	}

	m.extractedFacts = append(m.extractedFacts, sm.Facts...)
	m.lastExtractTime = time.Now()

	// Write to daily log
	now := time.Now()
	logPath := memory.GetAutoMemDailyLogPath(m.projectRoot, now.Year(), int(now.Month()), now.Day())

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n## Session extract — %s\n\n", now.Format("15:04")))
	for _, fact := range sm.Facts {
		sb.WriteString("- ")
		sb.WriteString(fact)
		sb.WriteString("\n")
	}

	if err := appendToFile(logPath, sb.String()); err != nil {
		slog.Warn("session memory: failed to write daily log",
			slog.String("path", logPath),
			slog.Any("err", err))
		return err
	}

	slog.Info("session memory: extracted and saved",
		slog.Int("facts", len(sm.Facts)),
		slog.String("log", logPath))

	return nil
}

// GetExtractedFacts returns all facts extracted in this session.
func (m *SessionMemoryManager) GetExtractedFacts() []string {
	return m.extractedFacts
}

// TurnCount returns the current turn count.
func (m *SessionMemoryManager) TurnCount() int {
	return m.turnCount
}

// ToolCallCount returns the current tool call count.
func (m *SessionMemoryManager) ToolCallCount() int {
	return m.toolCallCount
}

// ── SM-Compact pipeline extension ──────────────────────────────────────────

// RunSMCompact executes compaction with session memory extraction.
// It runs the standard pipeline but additionally:
// 1. Extracts session memories before compaction
// 2. Preserves recent messages after compaction
// 3. Injects extracted memories into the summary prefix
func RunSMCompact(
	ctx context.Context,
	prov provider.Provider,
	messages []*engine.Message,
	cfg PipelineConfig,
	smm *SessionMemoryManager,
) (*CompactionResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("sm-compact: no messages")
	}

	// Step 1: Extract session memories if conditions are met
	if smm != nil && smm.ShouldExtract() {
		if err := smm.ExtractAndSave(ctx, messages); err != nil {
			slog.Warn("sm-compact: memory extraction failed, continuing with compact",
				slog.Any("err", err))
		}
	}

	// Step 2: Split messages into compact-target and recent-keep
	keepCount := SMCompactRecentMessagesToKeep
	if keepCount > len(messages) {
		keepCount = len(messages)
	}
	toCompact := messages[:len(messages)-keepCount]
	toKeep := messages[len(messages)-keepCount:]

	if len(toCompact) == 0 {
		// Not enough messages to compact, just return
		return &CompactionResult{
			SummaryMessages:       messages,
			PreCompactTokenCount:  estimateTokensFromMessages(messages),
			PostCompactTokenCount: estimateTokensFromMessages(messages),
		}, nil
	}

	// Step 3: Run standard pipeline on the prefix
	newMsgs, pipeResult, err := RunPipeline(ctx, prov, toCompact, cfg)
	if err != nil {
		return nil, fmt.Errorf("sm-compact pipeline: %w", err)
	}

	// Step 4: Inject session memory context into summary
	if smm != nil && len(smm.extractedFacts) > 0 && len(newMsgs) > 0 {
		memoryNote := buildSessionMemoryNote(smm.extractedFacts)
		// Prepend to the first summary message
		first := newMsgs[0]
		if len(first.Content) > 0 && first.Content[0].Type == engine.ContentTypeText {
			first.Content[0].Text = memoryNote + "\n\n" + first.Content[0].Text
		}
	}

	// Step 5: Combine compacted prefix with preserved recent messages
	combined := append(newMsgs, toKeep...)

	preTokens := estimateTokensFromMessages(messages)
	postTokens := estimateTokensFromMessages(combined)

	cr := &CompactionResult{
		SummaryMessages:           combined,
		MessagesToKeep:            toKeep,
		PreCompactTokenCount:      preTokens,
		PostCompactTokenCount:     postTokens,
		TruePostCompactTokenCount: postTokens,
	}
	if pipeResult.CompactionResult != nil {
		cr.CompactionUsage = pipeResult.CompactionResult.CompactionUsage
	}

	return cr, nil
}

func buildSessionMemoryNote(facts []string) string {
	var sb strings.Builder
	sb.WriteString("[Session memory context]\n")
	for _, f := range facts {
		sb.WriteString("- ")
		sb.WriteString(f)
		sb.WriteString("\n")
	}
	return sb.String()
}

// ── File helpers ───────────────────────────────────────────────────────────

func appendToFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}
