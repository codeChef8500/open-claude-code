package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/wall-ai/agent-engine/internal/hooks"
)

const (
	maxTurns                     = 100
	maxOutputTokensRecoveryLimit = 3
)

// QueryTracking holds query chain tracking state.
type QueryTracking struct {
	ChainID string `json:"chain_id"`
	Depth   int    `json:"depth"`
}

// loopState tracks the internal state of a single query loop execution.
type loopState struct {
	messages     []*Message
	promptResult SystemPromptResult
	stopReason   string
	turnCount    int
	// tokenBudget is updated with real token counts from EventUsage events.
	tokenBudget TokenBudgetState
	// queryTracking tracks the query chain for analytics/debugging.
	queryTracking QueryTracking
	// maxOutputRecoveryCount tracks how many times we retried after truncation.
	maxOutputRecoveryCount int
	// stopHookActive is true when a stop hook forced a retry.
	stopHookActive bool
	// hookPreventedContinuation is set if a tool hook blocked continuation.
	hookPreventedContinuation bool
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
func runQueryLoop(ctx context.Context, e *Engine, params QueryParams, out chan<- *StreamEvent) error {
	qcfg := params.Config
	qdeps := params.Deps

	// ── 1. Load CLAUDE.md memory content ──────────────────────────────────
	var memoryContent string
	memLoader := e.memoryLoader
	if qdeps.MemoryLoader != nil {
		memLoader = qdeps.MemoryLoader
	}
	if memLoader != nil {
		if mc, err := memLoader.LoadMemory(e.cfg.WorkDir); err == nil {
			memoryContent = mc
		} else {
			slog.Debug("queryloop: memory load skipped", slog.Any("err", err))
		}
	}

	// ── 2. Build 6-layer system prompt ────────────────────────────────────
	effMaxTokens := resolveMaxTokens(e, qcfg)
	ls := &loopState{
		promptResult: buildSystemPromptIntegratedWithDeps(e, memoryContent, qdeps),
		tokenBudget: TokenBudgetState{
			ContextWindowSize:   effMaxTokens,
			CompactionThreshold: 0.85,
		},
	}

	// ── 3. Seed from prior session history ───────────────────────────────
	e.historyMu.Lock()
	ls.messages = append(ls.messages, e.history...)
	e.historyMu.Unlock()

	// ── 4. Persist user message ───────────────────────────────────────────
	userMsg := buildUserMessage(params)
	ls.messages = append(ls.messages, userMsg)
	e.persistMessage(userMsg)

	effModel := resolveModel(e, qcfg)
	effThinking := resolveThinkingBudget(e, qcfg)

	// Fire SessionStart hook.
	if e.hookExecutor != nil && e.hookExecutor.HasHooksFor(hooks.EventSessionStart) {
		e.hookExecutor.RunAsync(hooks.EventSessionStart, &hooks.HookInput{})
	}

	var loopErr error
	defer func() {
		// Fire SessionEnd hook.
		if e.hookExecutor != nil && e.hookExecutor.HasHooksFor(hooks.EventSessionEnd) {
			e.hookExecutor.RunAsync(hooks.EventSessionEnd, &hooks.HookInput{})
		}
	}()

	for ls.turnCount < maxTurns {
		ls.turnCount++
		e.session.IncrTurn()

		// Check abort.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// ── 5. Auto-compact using real token budget from EventUsage ───────
		if !qcfg.DisableCompaction && ls.tokenBudget.ShouldCompact() {
			slog.Info("queryloop: auto-compact triggered",
				slog.Int("turn", ls.turnCount),
				slog.String("session", e.cfg.SessionID),
				slog.Float64("usage_frac", ls.tokenBudget.UsageFraction()))
			// Fire PreCompact hook.
			if e.hookExecutor != nil && e.hookExecutor.HasHooksFor(hooks.EventPreCompact) {
				e.hookExecutor.RunSync(ctx, hooks.EventPreCompact, &hooks.HookInput{})
			}
			emitSystemMessage(out, "Compacting conversation to free context space…")
			compacted, _, err := CompactMessages(ctx, e.caller, ls.messages, effModel)
			if err != nil {
				slog.Warn("queryloop: auto-compact failed", slog.Any("err", err))
			} else {
				ls.messages = compacted
				ls.tokenBudget.InputTokens = 0 // reset after compact
			}
			// Fire PostCompact hook.
			if e.hookExecutor != nil && e.hookExecutor.HasHooksFor(hooks.EventPostCompact) {
				e.hookExecutor.RunSync(ctx, hooks.EventPostCompact, &hooks.HookInput{})
			}
		} else if !qcfg.DisableCompaction && effMaxTokens > 0 {
			// Fallback: estimate-based warning when real counts are not yet available.
			tokenEst := EstimateTokens(ls.messages)
			ratio := float64(tokenEst) / float64(effMaxTokens)
			if ratio >= 0.95 {
				emitSystemMessage(out, "Context window is nearly full. Triggering compaction…")
				if compacted, _, err := CompactMessages(ctx, e.caller, ls.messages, effModel); err == nil {
					ls.messages = compacted
				}
			} else if ratio >= WarningFraction {
				emitSystemMessage(out, "Context window is getting full. Consider using /compact.")
			}
		}

		toolDefs := e.toolDefsWithExtra(qdeps.ExtraTools)
		callParams := CallParams{
			Model:             effModel,
			MaxTokens:         effMaxTokens,
			ThinkingBudget:    effThinking,
			SystemPrompt:      ls.promptResult.Text,
			SystemPromptParts: ls.promptResult.Parts,
			Messages:          ls.messages,
			Tools:             toolDefs,
			UsePromptCache:    true,
		}

		eventCh, err := e.caller.CallModel(ctx, callParams)
		if err != nil {
			return fmt.Errorf("callModel: %w", err)
		}

		// Consume the event stream from the provider.
		assistantMsg, toolCalls, err := drainProviderStream(ctx, eventCh, out, e, &ls.tokenBudget)
		if err != nil {
			return err
		}

		// ── 6. Persist and append assistant turn ──────────────────────
		if assistantMsg != nil && len(assistantMsg.Content) > 0 {
			ls.messages = append(ls.messages, assistantMsg)
			e.persistMessage(assistantMsg)
		}

		// ── 7. Post-sampling hooks (fire-and-forget) ────────────────
		if e.hookExecutor != nil && e.hookExecutor.HasHooksFor(hooks.EventPostSampling) && assistantMsg != nil {
			contentJSON, _ := json.Marshal(assistantMsg.Content)
			e.hookExecutor.RunAsync(hooks.EventPostSampling, &hooks.HookInput{
				PostSampling: &hooks.PostSamplingInput{
					AssistantContent: contentJSON,
					StopReason:       ls.stopReason,
				},
			})
		}

		// ── 8. Handle abort during streaming ─────────────────────
		if ctx.Err() != nil {
			// Emit tombstones for any pending tool calls.
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

		// ── 9. No tool calls: model wants to stop ─────────────────
		if len(toolCalls) == 0 {
			// Max output tokens recovery: if the model was truncated, inject
			// a recovery message and retry.
			if ls.stopReason == "max_tokens" && ls.maxOutputRecoveryCount < maxOutputTokensRecoveryLimit {
				ls.maxOutputRecoveryCount++
				slog.Info("queryloop: max_tokens recovery",
					slog.Int("attempt", ls.maxOutputRecoveryCount))
				recoveryMsg := &Message{
					ID:   uuid.New().String(),
					Role: RoleUser,
					Content: []*ContentBlock{{
						Type: ContentTypeText,
						Text: "Output token limit hit. Resume directly \u2014 no apology, no recap of what you were doing. " +
							"Pick up mid-thought if that is where the cut happened. Break remaining work into smaller pieces.",
					}},
				}
				ls.messages = append(ls.messages, recoveryMsg)
				e.persistMessage(recoveryMsg)
				continue // retry
			}

			// Stop hooks: evaluate whether the model should really stop.
			if e.hookExecutor != nil && e.hookExecutor.HasHooksFor(hooks.EventStop) && !ls.stopHookActive {
				stopInput := &hooks.HookInput{
					Stop: &hooks.StopInput{
						StopReason: ls.stopReason,
					},
				}
				if assistantMsg != nil {
					for _, b := range assistantMsg.Content {
						if b.Type == ContentTypeText {
							stopInput.Stop.AssistantMessage = b.Text
							break
						}
					}
				}
				stopResp := e.hookExecutor.RunSync(ctx, hooks.EventStop, stopInput)
				if stopResp.Passed != nil && !*stopResp.Passed {
					// Hook says don't stop — inject failure reason and retry.
					slog.Info("queryloop: stop hook blocked", slog.String("reason", stopResp.FailureReason))
					// Fire StopFailure hook.
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
					continue // retry with stop hook feedback
				}
			}

			out <- &StreamEvent{Type: EventDone, SessionID: e.cfg.SessionID}
			return nil
		}

		// ── 10. Execute tool calls and append results ─────────────
		ls.stopHookActive = false // reset for next potential stop
		ls.maxOutputRecoveryCount = 0

		toolResultMsg, err := executeToolCalls(ctx, e, toolCalls, qdeps.ExtraTools, out)
		if err != nil {
			return err
		}
		ls.messages = append(ls.messages, toolResultMsg)
		e.persistMessage(toolResultMsg)

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

	loopErr = fmt.Errorf("exceeded maximum turn limit (%d)", maxTurns)
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
