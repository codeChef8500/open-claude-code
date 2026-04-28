package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/wall-ai/agent-engine/internal/hooks"
	"github.com/wall-ai/agent-engine/internal/util"
)

const (
	// defaultMaxTurns is the maximum number of tool-use turns before the loop exits.
	defaultMaxTurns = 100
	// maxOutputTokensRecoveryLimit is the max multi-turn recovery retries for max_output_tokens.
	maxOutputTokensRecoveryLimit = 3
	// escalatedMaxTokens is the output token limit used when OTK escalation is triggered.
	// Aligned with claude-code-main ESCALATED_MAX_TOKENS.
	escalatedMaxTokens = 64000
)

// QueryTracking holds query chain tracking state.
type QueryTracking struct {
	ChainID string `json:"chain_id"`
	Depth   int    `json:"depth"`
}

// loopState tracks the mutable state of a single query loop iteration.
// Aligned with the TS State type in query.ts:204-217.
// Each iteration may replace the entire loopState when continuing.
type loopState struct {
	// messages is the current conversation history for this iteration.
	messages []*Message

	// promptResult holds the built system prompt (populated once, reused).
	promptResult SystemPromptResult

	// stopReason is the last stop_reason from an assistant message.
	stopReason string

	// turnCount is the current turn number (1-indexed).
	turnCount int

	// tokenBudget is updated with real token counts from EventUsage events.
	tokenBudget TokenBudgetState

	// queryTracking tracks the query chain for analytics/debugging.
	queryTracking QueryTracking

	// ── Recovery & continuation state (aligned with TS State) ──────────

	// maxOutputTokensRecoveryCount tracks how many times we retried after
	// max_output_tokens truncation (cap: maxOutputTokensRecoveryLimit).
	maxOutputTokensRecoveryCount int

	// hasAttemptedReactiveCompact is true if reactive compact was tried
	// for prompt-too-long recovery this iteration.
	hasAttemptedReactiveCompact bool

	// maxOutputTokensOverride, if non-nil, overrides the default max output
	// tokens for the next API call (used by OTK escalation).
	maxOutputTokensOverride *int

	// pendingToolUseSummary receives a tool-use summary from the previous
	// iteration (generated async, consumed at the start of the next).
	pendingToolUseSummary <-chan *ToolUseSummaryMessage

	// stopHookActive is true when a stop hook forced a retry.
	stopHookActive bool

	// hookPreventedContinuation is set if a tool hook blocked continuation.
	hookPreventedContinuation bool

	// transition records why the previous iteration continued instead of
	// terminating. nil on the first iteration.
	transition *ContinueTransition

	// ── Auto-compact tracking ──────────────────────────────────────────

	// autoCompactTracking is per-chain tracking state for auto-compaction.
	autoCompactTracking *AutoCompactTrackingState
}

// AutoCompactTrackingState tracks compaction state across turns.
// Aligned with claude-code-main AutoCompactTrackingState.
type AutoCompactTrackingState struct {
	// Compacted is true if compaction has been performed in this chain.
	Compacted bool
	// TurnCounter counts turns since the last compaction.
	TurnCounter int
	// TurnID identifies this compaction chain for analytics.
	TurnID string
	// ConsecutiveFailures counts successive compaction failures.
	ConsecutiveFailures int
}

// ToolUseSummaryMessage is a message carrying a tool use summary.
type ToolUseSummaryMessage struct {
	// Summary is the human-readable summary text.
	Summary string
	// PrecedingToolUseIDs are the tool_use IDs this summary covers.
	PrecedingToolUseIDs []string
}

// resolveModel returns the effective model name for this query.
func resolveModel(e *Engine, cfg QueryConfig) string {
	if cfg.Model != "" {
		return cfg.Model
	}
	return e.cfg.Model
}

// resolveMaxTokens returns the effective max-tokens for this query.
func resolveMaxTokens(e *Engine, cfg QueryConfig) int {
	if cfg.MaxTokens > 0 {
		return cfg.MaxTokens
	}
	return e.cfg.MaxTokens
}

// resolveThinkingBudget returns the effective thinking budget for this query.
func resolveThinkingBudget(e *Engine, cfg QueryConfig) int {
	if cfg.ThinkingBudget > 0 {
		return cfg.ThinkingBudget
	}
	return e.cfg.ThinkingBudget
}

// runQueryLoop is the core for-select state machine that drives the
// conversation: callModel → dispatch tool calls → continue or stop.
// It wires all standalone modules (prefetch, compression pipeline, fallback,
// withheld error recovery, stop hooks handler, token budget continuation,
// streaming tool execution, task budget, transcript, transitions).
func runQueryLoop(ctx context.Context, e *Engine, params QueryParams, out chan<- *StreamEvent) error {
	qcfg := params.Config
	qdeps := params.Deps

	// ── 1. Snapshot immutable config & resolve effective values ────────────
	qlCfg := BuildQueryLoopConfig(e.cfg.SessionID, e.featureFlags)
	effMaxTokens := resolveMaxTokens(e, qcfg)
	effModel := resolveModel(e, qcfg)
	effThinking := resolveThinkingBudget(e, qcfg)

	// ── 2. Concurrent prefetch: memory + attachments ──────────────────────
	prefetchResult := RunPrefetch(ctx, e, &qdeps, DefaultPrefetchConfig())
	memoryContent := prefetchResult.MemoryContent

	// ── 3. Build 6-layer system prompt ────────────────────────────────────
	ls := &loopState{
		promptResult: buildSystemPromptIntegratedWithDeps(e, memoryContent, qdeps),
		tokenBudget: TokenBudgetState{
			ContextWindowSize:   effMaxTokens,
			CompactionThreshold: 0.85,
		},
	}

	// ── 4. Initialize subsystems from existing modules ────────────────────
	// ToolUseContext — carries all tool execution state for the query.
	toolUseCtx := NewToolUseContext(ctx, e, e.enabledTools(), effModel, &ls.queryTracking)
	// Cumulative usage tracker.
	cumUsage := &CumulativeUsage{}
	// Task budget tracker (USD/token/time).
	var taskBudget *TaskBudgetTracker
	if qcfg.MaxBudgetUSD != nil && *qcfg.MaxBudgetUSD > 0 {
		taskBudget = NewTaskBudgetTracker(*qcfg.MaxBudgetUSD, 0, qcfg.Timeout)
	}
	// Token budget continuation tracker (diminishing returns heuristic).
	budgetContinuation := NewBudgetContinuationTracker()
	// Transcript for session lifecycle.
	transcript := NewTranscript(e.cfg.SessionID)
	// Stop hooks handler — full pipeline with post-sampling hooks.
	stopHandler := NewStopHooksHandler(nil, nil) // wired with actual hooks below
	if e.hookExecutor != nil {
		// We pass nil for the StopHookExecutor and PostSamplingHooks here;
		// the actual hooks executor integration uses the inline hookExecutor
		// path below for backward compatibility until stop hook configs are
		// plumbed through EngineConfig.
		// stopHandler is used in the main loop for HandleStopReason calls.
	}

	// Recovery config for withheld error handling.
	recoveryCfg := RecoveryConfig{
		Model:                  effModel,
		ContextWindowSize:      effMaxTokens,
		ReactiveCompactEnabled: e.featureFlags != nil && e.featureFlags.IsEnabled(util.FlagReactiveCompact),
	}

	// ── 5. Seed from prior session history ───────────────────────────────
	e.historyMu.Lock()
	ls.messages = append(ls.messages, e.history...)
	e.historyMu.Unlock()

	// ── 6. Persist user message ───────────────────────────────────────────
	userMsg := buildUserMessage(params)
	ls.messages = append(ls.messages, userMsg)
	e.persistMessage(userMsg)
	transcript.Append(userMsg, "user", 0, 0)

	// Suppress unused variable warnings for subsystems used in later phases.
	_ = toolUseCtx

	// Fire SessionStart hook.
	if e.hookExecutor != nil && e.hookExecutor.HasHooksFor(hooks.EventSessionStart) {
		e.hookExecutor.RunAsync(hooks.EventSessionStart, &hooks.HookInput{})
	}

	// ── Session lifecycle: record query start ────────────────────────────
	sessState := NewSessionState(e.cfg.SessionID)
	sessState.RecordQueryStart(effModel)

	var loopErr error
	defer func() {
		// ── Session lifecycle: record query end ──────────────────────────
		sessState.RecordQueryEnd(cumUsage.ToUsageStats())

		// Emit final usage summary to the stream.
		YieldUsage(cumUsage.ToUsageStats(), out)

		// Update engine-level session usage stats.
		e.session.AddUsage(
			cumUsage.TotalInputTokens,
			cumUsage.TotalOutputTokens,
			cumUsage.TotalCostUSD,
		)

		// Log transcript summary.
		slog.Info("queryloop: session complete",
			slog.String("session_id", e.cfg.SessionID),
			slog.Int("turns", ls.turnCount),
			slog.Int("transcript_entries", transcript.Len()),
			slog.Duration("duration", transcript.Duration()),
			slog.Int("total_input_tokens", cumUsage.TotalInputTokens),
			slog.Int("total_output_tokens", cumUsage.TotalOutputTokens),
			slog.Float64("total_cost_usd", cumUsage.TotalCostUSD))

		// Fire SessionEnd hook.
		if e.hookExecutor != nil && e.hookExecutor.HasHooksFor(hooks.EventSessionEnd) {
			e.hookExecutor.RunAsync(hooks.EventSessionEnd, &hooks.HookInput{})
		}
	}()

	effMaxTurns := defaultMaxTurns
	if qcfg.MaxTurns > 0 {
		effMaxTurns = qcfg.MaxTurns
	}
	for ls.turnCount < effMaxTurns {
		ls.turnCount++
		e.session.IncrTurn()

		// Check abort.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// ── 7. Compression pipeline (5-step) ─────────────────────────────
		// Delegates to RunCompressionPipeline which runs: boundary trim,
		// tool result budget, history snip, micro-compact, auto-compact.
		pipelineCfg := CompressionPipelineConfig{
			DisableCompaction: qcfg.DisableCompaction,
			QuerySource:       params.Source,
			Flags:             e.featureFlags,
			Model:             effModel,
			ContextWindowSize: effMaxTokens,
		}

		// If real token budget shows the context is full, or estimate-based
		// fallback detects it, the pipeline handles both cases internally.
		shouldRunPipeline := !qcfg.DisableCompaction &&
			(ls.tokenBudget.ShouldCompact() || (effMaxTokens > 0 && ls.turnCount > 1))

		if shouldRunPipeline {
			// Fire PreCompact hook before any compression work.
			if e.hookExecutor != nil && e.hookExecutor.HasHooksFor(hooks.EventPreCompact) {
				e.hookExecutor.RunSync(ctx, hooks.EventPreCompact, &hooks.HookInput{})
			}

			pResult, pErr := RunCompressionPipeline(ctx, ls.messages, e.caller, &qdeps, pipelineCfg)
			if pErr != nil {
				slog.Warn("queryloop: compression pipeline error", slog.Any("err", pErr))
			} else {
				ls.messages = pResult.Messages
				// Emit snip boundary if the pipeline performed a snip.
				if pResult.SnipBoundaryMessage != nil {
					ls.messages = append(ls.messages, pResult.SnipBoundaryMessage)
				}
				// Reset real token budget after compaction.
				if pResult.CompactionResult != nil {
					ls.tokenBudget.InputTokens = 0
					emitSystemMessage(out, "Compacting conversation to free context space…")
				}
			}

			// Fire PostCompact hook after compression work.
			if e.hookExecutor != nil && e.hookExecutor.HasHooksFor(hooks.EventPostCompact) {
				e.hookExecutor.RunSync(ctx, hooks.EventPostCompact, &hooks.HookInput{})
			}
		} else if !qcfg.DisableCompaction && effMaxTokens > 0 && ls.turnCount == 1 {
			// First turn: run lightweight pipeline (boundary + tool result budget only).
			pResult, _ := RunCompressionPipeline(ctx, ls.messages, e.caller, &qdeps, pipelineCfg)
			if pResult != nil {
				ls.messages = pResult.Messages
			}
		}

		// Apply max output token override (from OTK escalation recovery).
		effCallMaxTokens := effMaxTokens
		if ls.maxOutputTokensOverride != nil {
			effCallMaxTokens = *ls.maxOutputTokensOverride
		}

		toolDefs := e.toolDefsWithExtra(qdeps.ExtraTools)
		callParams := CallParams{
			Model:             effModel,
			MaxTokens:         effCallMaxTokens,
			ThinkingBudget:    effThinking,
			SystemPrompt:      ls.promptResult.Text,
			SystemPromptParts: ls.promptResult.Parts,
			Messages:          ls.messages,
			Tools:             toolDefs,
			UsePromptCache:    true,
		}

		eventCh, err := e.caller.CallModel(ctx, callParams)

		// ── 8. Fallback error handling ────────────────────────────
		// If CallModel returns a FallbackTriggeredError, switch models,
		// emit tombstones, and retry the iteration.
		if err != nil {
			if fte, ok := IsFallbackTriggeredError(err); ok && qcfg.FallbackModel != "" {
				cleaned, newModel := HandleFallback(fte, ls.messages, nil, out)
				ls.messages = cleaned
				effModel = newModel
				ls.transition = &ContinueTransition{Reason: ContinueFallbackRetry}
				continue
			}
			return fmt.Errorf("callModel: %w", err)
		}

		// Consume the event stream from the provider.
		assistantMsg, toolCalls, streamErr := drainProviderStream(ctx, eventCh, out, e, &ls.tokenBudget)

		// Handle streaming fallback error.
		if streamErr != nil {
			if fte, ok := IsFallbackTriggeredError(streamErr); ok && qcfg.FallbackModel != "" {
				var emittedUUIDs []string
				if assistantMsg != nil && assistantMsg.UUID != "" {
					emittedUUIDs = append(emittedUUIDs, assistantMsg.UUID)
				}
				cleaned, newModel := HandleFallback(fte, ls.messages, emittedUUIDs, out)
				ls.messages = cleaned
				effModel = newModel
				ls.transition = &ContinueTransition{Reason: ContinueFallbackRetry}
				continue
			}
			return streamErr
		}

		// ── 9. Normalize & persist assistant turn ─────────────────────
		if assistantMsg != nil && len(assistantMsg.Content) > 0 {
			assistantMsg = NormalizeAssistantMessage(assistantMsg)
			ls.stopReason = assistantMsg.StopReason
			ls.messages = append(ls.messages, assistantMsg)
			e.persistMessage(assistantMsg)
			transcript.Append(assistantMsg, "assistant", 0, 0)
		}

		// ── 10. Post-sampling hooks (fire-and-forget) ────────────────
		if e.hookExecutor != nil && e.hookExecutor.HasHooksFor(hooks.EventPostSampling) && assistantMsg != nil {
			contentJSON, _ := json.Marshal(assistantMsg.Content)
			e.hookExecutor.RunAsync(hooks.EventPostSampling, &hooks.HookInput{
				PostSampling: &hooks.PostSamplingInput{
					AssistantContent: contentJSON,
					StopReason:       ls.stopReason,
				},
			})
		}

		// ── 11. Withheld error detection & recovery ──────────────────
		// Use DetectWithheldError to classify the assistant message, then
		// delegate to the appropriate recovery handler.
		if assistantMsg != nil {
			withheldType := DetectWithheldError(assistantMsg)
			switch withheldType {
			case WithheldPromptTooLong:
				action := HandleWithheldPromptTooLong(ctx, ls, e.caller, recoveryCfg, out)
				if action.IsFatal {
					return action.FatalError
				}
				if action.SystemMessage != "" {
					emitSystemMessage(out, action.SystemMessage)
				}
				ls.messages = action.Messages
				ls.transition = action.Transition
				if action.MaxOutputTokensOverride != nil {
					ls.maxOutputTokensOverride = action.MaxOutputTokensOverride
				}
				continue

			case WithheldMaxOutputTokens:
				action := HandleWithheldMaxOutputTokens(ls, effCallMaxTokens)
				if action.IsFatal {
					// Fall through to normal stop handling — cannot recover.
					slog.Warn("queryloop: max_output_tokens recovery exhausted")
				} else {
					if action.SystemMessage != "" {
						emitSystemMessage(out, action.SystemMessage)
					}
					ls.messages = action.Messages
					ls.transition = action.Transition
					if action.MaxOutputTokensOverride != nil {
						ls.maxOutputTokensOverride = action.MaxOutputTokensOverride
					}
					continue
				}

			case WithheldMediaSizeError:
				action := HandleWithheldMediaSizeError(ctx, ls, e.caller, recoveryCfg, out)
				if action.IsFatal {
					return action.FatalError
				}
				if action.SystemMessage != "" {
					emitSystemMessage(out, action.SystemMessage)
				}
				ls.messages = action.Messages
				ls.transition = action.Transition
				continue
			}
		}

		// ── 12. Handle abort during streaming ─────────────────────
		if ctx.Err() != nil {
			for _, tc := range toolCalls {
				out <- &StreamEvent{
					Type:     EventToolResult,
					ToolID:   tc.ID,
					ToolName: tc.Name,
					IsError:  true,
					Text:     "[Interrupted by user]",
				}
			}
			out <- &StreamEvent{Type: EventDone, SessionID: e.cfg.SessionID}
			return ctx.Err()
		}

		// ── 13. No tool calls: model wants to stop ─────────────────
		if len(toolCalls) == 0 {
			// ── 13a. Stop hooks pipeline ─────────────────────────────
			// Build context and delegate to StopHooksHandler for the full
			// pipeline: user hooks → post-sampling hooks → blocking retry.
			hookCtx := &StopHookContext{
				StopReason:       ls.stopReason,
				AssistantMessage: assistantMsg,
				Messages:         ls.messages,
				TurnCount:        ls.turnCount,
				Model:            effModel,
				SessionID:        e.cfg.SessionID,
				HasToolUse:       false,
				QuerySource:      params.Source,
			}

			decision := stopHandler.HandleStopReason(ctx, hookCtx)

			// If a blocking hook said "continue", inject the blocking message.
			if decision.ShouldContinue && decision.BlockingMessage != nil {
				ls.messages = append(ls.messages, decision.BlockingMessage)
				e.persistMessage(decision.BlockingMessage)
				ls.stopHookActive = true
				ls.transition = &ContinueTransition{Reason: ContinueStopHookRetry}
				continue
			}

			// Also run legacy hookExecutor path for backward compat.
			if e.hookExecutor != nil && e.hookExecutor.HasHooksFor(hooks.EventStop) && !ls.stopHookActive {
				stopInput := &hooks.HookInput{
					Stop: &hooks.StopInput{
						StopReason: ls.stopReason,
					},
				}
				if assistantMsg != nil {
					stopInput.Stop.AssistantMessage = ExtractAssistantText(assistantMsg)
				}
				stopResp := e.hookExecutor.RunSync(ctx, hooks.EventStop, stopInput)
				if stopResp.Passed != nil && !*stopResp.Passed {
					slog.Info("queryloop: stop hook blocked", slog.String("reason", stopResp.FailureReason))
					if e.hookExecutor.HasHooksFor(hooks.EventStopFailure) {
						e.hookExecutor.RunAsync(hooks.EventStopFailure, stopInput)
					}
					failMsg := &Message{
						ID:   uuid.New().String(),
						Role: RoleUser,
						Content: []*ContentBlock{{
							Type: ContentTypeText,
							Text: fmt.Sprintf("Stop hook failed: %s. Please address this before completing.", stopResp.FailureReason),
						}},
					}
					ls.messages = append(ls.messages, failMsg)
					e.persistMessage(failMsg)
					ls.stopHookActive = true
					ls.transition = &ContinueTransition{Reason: ContinueStopHookRetry}
					continue
				}
			}

			// ── 13b. Token budget continuation ───────────────────────
			// If the model stopped with end_turn but a task budget is active,
			// check if we should auto-continue (the model may have more work).
			if params.TaskBudget != nil && ls.stopReason == "end_turn" {
				tbDecision := CheckTokenBudgetContinuation(
					budgetContinuation,
					ls.tokenBudget.OutputTokens,
					effMaxTokens,
					ls.stopReason,
				)
				if tbDecision.ShouldContinue {
					slog.Info("queryloop: token budget continuation",
						slog.Int("count", tbDecision.ContinuationCount),
						slog.String("reason", tbDecision.Reason))
					contMsg := BuildContinuationMessage(tbDecision.ContinuationCount)
					ls.messages = append(ls.messages, contMsg)
					e.persistMessage(contMsg)
					ls.transition = &ContinueTransition{Reason: ContinueTokenBudgetContinuation}
					continue
				}
			}

			out <- &StreamEvent{Type: EventDone, SessionID: e.cfg.SessionID}
			return nil
		}

		// ── 14. Execute tool calls and append results ─────────────
		ls.stopHookActive = false
		ls.maxOutputTokensRecoveryCount = 0
		ls.transition = nil

		var toolResultMsg *Message

		// Use streaming tool execution if the gate is enabled and we have
		// concurrent-safe tool implementations.
		if ShouldUseStreamingToolExec(qlCfg.Gates, len(toolCalls)) {
			// Convert ToolCall list into ContentBlock-based requests.
			var blocks []*ContentBlock
			for _, tc := range toolCalls {
				blocks = append(blocks, &ContentBlock{
					Type:      ContentTypeToolUse,
					ToolName:  tc.Name,
					ToolUseID: tc.ID,
					Input:     tc.Input,
				})
			}
			stExec := NewStreamingToolExecutor(NewToolExecutor(nil), 4, func(ev StreamEvent) {
				out <- &ev
			})
			collector := NewStreamingToolCollector(ctx, stExec, e.enabledTools())
			for _, b := range blocks {
				collector.AddTool(b, assistantMsg)
			}
			results := collector.GetRemainingResults()
			toolResultMsg = BuildToolResultsFromStreaming(results)
		} else {
			var execErr error
			toolResultMsg, execErr = executeToolCalls(ctx, e, toolCalls, qdeps.ExtraTools, out)
			if execErr != nil {
				return execErr
			}
		}

		toolResultMsg = NormalizeToolResultMessage(toolResultMsg)
		ls.messages = append(ls.messages, toolResultMsg)
		e.persistMessage(toolResultMsg)
		transcript.Append(toolResultMsg, "tool_result", 0, 0)

		// ── 15. Task budget enforcement ──────────────────────────
		// After tool execution, check if the task budget is exhausted.
		if taskBudget != nil {
			// Record usage from this turn.
			taskBudget.Record(0, ls.tokenBudget.OutputTokens)
			if taskBudget.IsExhausted() {
				reason := taskBudget.ExhaustionReason()
				slog.Info("queryloop: task budget exhausted", slog.String("reason", reason))
				emitSystemMessage(out, "Task budget exhausted: "+reason)
				out <- &StreamEvent{Type: EventDone, SessionID: e.cfg.SessionID}
				return nil
			}
		}

		// Check if a hook prevented continuation.
		if ls.hookPreventedContinuation {
			out <- &StreamEvent{Type: EventDone, SessionID: e.cfg.SessionID}
			return nil
		}

		// Check abort after tool execution.
		if ctx.Err() != nil {
			out <- &StreamEvent{Type: EventDone, SessionID: e.cfg.SessionID}
			return ctx.Err()
		}
	}

	loopErr = fmt.Errorf("exceeded maximum turn limit (%d)", effMaxTurns)
	emitSystemMessage(out, fmt.Sprintf("Reached maximum turn limit (%d). Stopping.", effMaxTurns))
	out <- &StreamEvent{Type: EventDone, SessionID: e.cfg.SessionID}

	slog.Warn("queryloop: max turns reached",
		slog.Int("max_turns", effMaxTurns),
		slog.Int("completed", ls.turnCount))

	return loopErr
}

// buildSystemPromptIntegratedWithDeps builds the system prompt, honouring any
// per-query overrides in QueryDeps.
func buildSystemPromptIntegratedWithDeps(e *Engine, memoryContent string, deps QueryDeps) SystemPromptResult {
	builder := e.promptBuilder
	if deps.SystemPromptBuilder != nil {
		builder = deps.SystemPromptBuilder
	}
	tools := e.enabledTools()
	if len(deps.ExtraTools) > 0 {
		tools = append(tools, deps.ExtraTools...)
	}
	append_ := e.cfg.AppendSystemPrompt
	if deps.ExtraSystemPrompt != "" {
		if append_ != "" {
			append_ += "\n\n" + deps.ExtraSystemPrompt
		} else {
			append_ = deps.ExtraSystemPrompt
		}
	}
	if builder != nil {
		return builder.BuildParts(SystemPromptOptions{
			Tools:              tools,
			UseContext:         e.useContext(),
			WorkDir:            e.cfg.WorkDir,
			MemoryContent:      memoryContent,
			CustomSystemPrompt: e.cfg.CustomSystemPrompt,
			AppendSystemPrompt: append_,
		})
	}
	return SystemPromptResult{Text: buildSystemPrompt(e)}
}

// drainProviderStream reads events from the provider channel, forwards
// text/thinking/usage events to `out`, accumulates the assistant message,
// and returns any pending tool calls.
func drainProviderStream(
	ctx context.Context,
	eventCh <-chan *StreamEvent,
	out chan<- *StreamEvent,
	e *Engine,
	budget *TokenBudgetState,
) (*Message, []*pendingToolCall, error) {

	assistantMsg := &Message{
		ID:   uuid.New().String(),
		Role: RoleAssistant,
	}

	var toolCalls []*pendingToolCall
	// map toolID -> accumulated input JSON
	toolInputBuf := make(map[string]*json.RawMessage)

	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case ev, ok := <-eventCh:
			if !ok {
				// Flush accumulated tool-input buffers back into each pendingToolCall
				// so that executeToolCalls receives the complete JSON input.
				for _, tc := range toolCalls {
					if buf, exists := toolInputBuf[tc.ID]; exists && buf != nil {
						tc.Input = *buf
					}
				}
				return assistantMsg, toolCalls, nil
			}
			switch ev.Type {
			case EventTextDelta:
				// Accumulate text for the assistant message.
				appendTextToMessage(assistantMsg, ev.Text)
				// Forward to caller.
				out <- ev

			case EventThinking:
				appendThinkingToMessage(assistantMsg, ev.Thinking)
				out <- ev

			case EventToolUse:
				// A new tool call has started.
				tc := &pendingToolCall{
					ID:   ev.ToolID,
					Name: ev.ToolName,
				}
				toolCalls = append(toolCalls, tc)
				// Pre-allocate input buffer.
				empty := json.RawMessage("{}")
				toolInputBuf[ev.ToolID] = &empty
				// If input arrived in one shot (non-streaming), capture it.
				if ev.ToolInput != nil {
					b, _ := json.Marshal(ev.ToolInput)
					raw := json.RawMessage(b)
					toolInputBuf[ev.ToolID] = &raw
				}
				// Add tool_use block to assistant message.
				assistantMsg.Content = append(assistantMsg.Content, &ContentBlock{
					Type:      ContentTypeToolUse,
					ToolUseID: ev.ToolID,
					ToolName:  ev.ToolName,
					Input:     ev.ToolInput,
				})
				out <- ev

			case EventUsage:
				if ev.Usage != nil {
					costUSD := computeCostUSD(ev.Usage, e.cfg.Model)
					ev.Usage.CostUSD = costUSD
					e.session.AddUsage(ev.Usage.InputTokens, ev.Usage.OutputTokens, costUSD)
					e.store.AddCostUSD(costUSD)
					// Update real token budget from provider response.
					if budget != nil {
						budget.InputTokens = ev.Usage.InputTokens
						budget.OutputTokens += ev.Usage.OutputTokens
						budget.CacheReadTokens += ev.Usage.CacheReadInputTokens
						budget.CacheWriteTokens += ev.Usage.CacheCreationInputTokens
					}
				}
				out <- ev

			case EventError:
				return nil, nil, fmt.Errorf("provider error: %s", ev.Error)

			case EventDone:
				// EventDone from the provider is a hint that the stream is ending;
				// the loop exits naturally when the channel closes.

			default:
				slog.Debug("unknown stream event", slog.String("type", string(ev.Type)))
			}
		}
	}
}

// pendingToolCall is an accumulator for a single tool call during streaming.
type pendingToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// toolCallResult holds the outcome of a single tool execution.
type toolCallResult struct {
	toolUseID string
	blocks    []*ContentBlock
	isErr     bool
	toolName  string
	denied    bool
	deniedMsg string
}

// tombstoneResult returns a tool-result block that documents an interrupted tool call.
func tombstoneResult(toolUseID, toolName string) *ContentBlock {
	return &ContentBlock{
		Type:      ContentTypeToolResult,
		ToolUseID: toolUseID,
		Content: []*ContentBlock{{
			Type: ContentTypeText,
			Text: fmt.Sprintf("[Tool %s was interrupted before it could complete.]", toolName),
		}},
		IsError: true,
	}
}

// executeToolCalls runs pending tool calls: concurrent-safe tools are
// dispatched in parallel; the rest run sequentially in order.
func executeToolCalls(
	ctx context.Context,
	e *Engine,
	calls []*pendingToolCall,
	extraTools []Tool,
	out chan<- *StreamEvent,
) (*Message, error) {

	resultMsg := &Message{
		ID:   uuid.New().String(),
		Role: RoleUser,
	}

	// Pre-check permissions and group by concurrency safety.
	type group struct {
		tc   *pendingToolCall
		tool Tool
	}
	var concurrent []group
	var sequential []group

	for _, tc := range calls {
		// If context is already cancelled, tombstone remaining calls.
		if ctx.Err() != nil {
			resultMsg.Content = append(resultMsg.Content, tombstoneResult(tc.ID, tc.Name))
			continue
		}
		t, ok := e.findToolWithExtra(tc.Name, extraTools)
		if !ok {
			resultMsg.Content = append(resultMsg.Content, &ContentBlock{
				Type:      ContentTypeToolResult,
				ToolUseID: tc.ID,
				Content:   []*ContentBlock{{Type: ContentTypeText, Text: fmt.Sprintf("tool not found: %s", tc.Name)}},
				IsError:   true,
			})
			continue
		}
		uctx := e.useContext()

		// Global permission check.
		if e.permChecker != nil {
			verdict, reason := e.permChecker.CheckTool(ctx, tc.Name, tc.Input, e.cfg.WorkDir)
			if verdict == PermissionDeny {
				emitSystemMessage(out, fmt.Sprintf("Permission denied for %s: %s", tc.Name, reason))
				resultMsg.Content = append(resultMsg.Content, &ContentBlock{
					Type:      ContentTypeToolResult,
					ToolUseID: tc.ID,
					Content:   []*ContentBlock{{Type: ContentTypeText, Text: "Permission denied: " + reason}},
					IsError:   true,
				})
				continue
			}
		}

		// Auto Mode LLM classifier.
		if e.cfg.AutoMode && e.autoModeClassifier != nil {
			verdict, reason, err := e.autoModeClassifier.Classify(ctx, tc.Name, tc.Input)
			if err != nil {
				slog.Warn("auto mode classifier error", slog.Any("err", err))
			} else if verdict == PermissionDeny {
				emitSystemMessage(out, fmt.Sprintf("Auto Mode denied %s: %s", tc.Name, reason))
				resultMsg.Content = append(resultMsg.Content, &ContentBlock{
					Type:      ContentTypeToolResult,
					ToolUseID: tc.ID,
					Content:   []*ContentBlock{{Type: ContentTypeText, Text: fmt.Sprintf("Auto Mode denied: %s", reason)}},
					IsError:   true,
				})
				continue
			} else if verdict == PermissionSoftDeny {
				emitSystemMessage(out, fmt.Sprintf("Auto Mode soft-denied %s (proceeding with caution): %s", tc.Name, reason))
			}
		}

		// Per-tool permission check.
		if err := t.CheckPermissions(ctx, tc.Input, uctx); err != nil {
			resultMsg.Content = append(resultMsg.Content, &ContentBlock{
				Type:      ContentTypeToolResult,
				ToolUseID: tc.ID,
				Content:   []*ContentBlock{{Type: ContentTypeText, Text: "Permission denied: " + err.Error()}},
				IsError:   true,
			})
			continue
		}

		g := group{tc: tc, tool: t}
		if t.IsConcurrencySafe(tc.Input) {
			concurrent = append(concurrent, g)
		} else {
			sequential = append(sequential, g)
		}
	}

	// If ctx is already cancelled at this point, tombstone everything pending.
	if ctx.Err() != nil {
		for _, g := range concurrent {
			resultMsg.Content = append(resultMsg.Content, tombstoneResult(g.tc.ID, g.tc.Name))
		}
		for _, g := range sequential {
			resultMsg.Content = append(resultMsg.Content, tombstoneResult(g.tc.ID, g.tc.Name))
		}
		return resultMsg, nil
	}

	// Execute concurrent-safe tools in parallel.
	if len(concurrent) > 0 {
		results := make([]toolCallResult, len(concurrent))
		var wg sync.WaitGroup
		for i, g := range concurrent {
			wg.Add(1)
			go func(idx int, g group) {
				defer wg.Done()
				results[idx] = runSingleTool(ctx, g.tc, g.tool, e)
			}(i, g)
		}
		wg.Wait()

		for _, res := range results {
			out <- &StreamEvent{
				Type:     EventToolResult,
				ToolID:   res.toolUseID,
				ToolName: res.toolName,
				Text:     blocksToString(res.blocks),
				IsError:  res.isErr,
			}
			resultMsg.Content = append(resultMsg.Content, &ContentBlock{
				Type:      ContentTypeToolResult,
				ToolUseID: res.toolUseID,
				Content:   res.blocks,
				IsError:   res.isErr,
			})
		}
	}

	// Execute sequential tools one at a time.
	for _, g := range sequential {
		// Tombstone if context cancelled between sequential calls.
		if ctx.Err() != nil {
			resultMsg.Content = append(resultMsg.Content, tombstoneResult(g.tc.ID, g.tc.Name))
			continue
		}
		res := runSingleTool(ctx, g.tc, g.tool, e)
		// If tool returned no blocks (e.g. cancelled), inject tombstone.
		if len(res.blocks) == 0 && ctx.Err() != nil {
			resultMsg.Content = append(resultMsg.Content, tombstoneResult(g.tc.ID, g.tc.Name))
			continue
		}
		out <- &StreamEvent{
			Type:     EventToolResult,
			ToolID:   res.toolUseID,
			ToolName: res.toolName,
			Text:     blocksToString(res.blocks),
			IsError:  res.isErr,
		}
		resultMsg.Content = append(resultMsg.Content, &ContentBlock{
			Type:      ContentTypeToolResult,
			ToolUseID: res.toolUseID,
			Content:   res.blocks,
			IsError:   res.isErr,
		})
	}

	return resultMsg, nil
}

// runSingleTool executes one tool call and collects its result blocks.
func runSingleTool(ctx context.Context, tc *pendingToolCall, t Tool, e *Engine) toolCallResult {
	uctx := e.useContext()
	blockCh, err := t.Call(ctx, tc.Input, uctx)
	if err != nil {
		return toolCallResult{
			toolUseID: tc.ID,
			toolName:  tc.Name,
			blocks:    []*ContentBlock{{Type: ContentTypeText, Text: err.Error()}},
			isErr:     true,
		}
	}
	var blocks []*ContentBlock
	isErr := false
	for b := range blockCh {
		if b.IsError {
			isErr = true
		}
		blocks = append(blocks, b)
	}
	return toolCallResult{
		toolUseID: tc.ID,
		toolName:  tc.Name,
		blocks:    blocks,
		isErr:     isErr,
	}
}

// buildSystemPrompt assembles the system prompt from engine config.
// Full 6-layer assembly is in the prompt package; here we use a lightweight
// version that can be replaced via dependency injection.
func buildSystemPrompt(e *Engine) string {
	// Base system prompt — callers can inject a full prompt via config.
	base := "You are Claude, an AI assistant made by Anthropic."
	if e.cfg.CustomSystemPrompt != "" {
		return e.cfg.CustomSystemPrompt
	}
	if e.cfg.AppendSystemPrompt != "" {
		return base + "\n\n" + e.cfg.AppendSystemPrompt
	}
	return base
}

// buildUserMessage converts QueryParams into an internal Message.
func buildUserMessage(params QueryParams) *Message {
	var blocks []*ContentBlock
	if params.Text != "" {
		blocks = append(blocks, &ContentBlock{Type: ContentTypeText, Text: params.Text})
	}
	for _, imgData := range params.Images {
		blocks = append(blocks, &ContentBlock{
			Type:      ContentTypeImage,
			MediaType: "image/png", // caller should specify correct media type
			Data:      imgData,
		})
	}
	return &Message{
		ID:      uuid.New().String(),
		Role:    RoleUser,
		Content: blocks,
	}
}

// appendTextToMessage finds or creates a text block in assistantMsg and appends text.
func appendTextToMessage(msg *Message, text string) {
	for i := len(msg.Content) - 1; i >= 0; i-- {
		if msg.Content[i].Type == ContentTypeText {
			msg.Content[i].Text += text
			return
		}
	}
	msg.Content = append(msg.Content, &ContentBlock{Type: ContentTypeText, Text: text})
}

// appendThinkingToMessage appends thinking text to the last thinking block
// or creates a new one.
func appendThinkingToMessage(msg *Message, thinking string) {
	for i := len(msg.Content) - 1; i >= 0; i-- {
		if msg.Content[i].Type == ContentTypeThinking {
			msg.Content[i].Thinking += thinking
			return
		}
	}
	msg.Content = append(msg.Content, &ContentBlock{Type: ContentTypeThinking, Thinking: thinking})
}
