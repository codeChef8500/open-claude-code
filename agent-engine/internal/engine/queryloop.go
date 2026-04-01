package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// compactThreshold is the fraction of MaxTokens at which auto-compact triggers.
const compactThreshold = 0.80

const maxTurns = 100

// loopState tracks the internal state of a single query loop execution.
type loopState struct {
	messages    []*Message
	systemPrompt string
	stopReason  string
	turnCount   int
}

// runQueryLoop is the core for-select state machine that drives the
// conversation: callModel → dispatch tool calls → continue or stop.
func runQueryLoop(ctx context.Context, e *Engine, params QueryParams, out chan<- *StreamEvent) error {
	// ── 1. Load CLAUDE.md memory content ──────────────────────────────────
	var memoryContent string
	if e.memoryLoader != nil {
		if mc, err := e.memoryLoader.LoadMemory(e.cfg.WorkDir); err == nil {
			memoryContent = mc
		} else {
			slog.Debug("queryloop: memory load skipped", slog.Any("err", err))
		}
	}

	// ── 2. Build 6-layer system prompt ────────────────────────────────────
	ls := &loopState{
		systemPrompt: buildSystemPromptIntegrated(e, memoryContent),
	}

	// ── 3. Seed from prior session history if resuming ────────────────────
	// (session.resume wires prior messages before SubmitMessage is called)

	// ── 4. Persist user message ───────────────────────────────────────────
	userMsg := buildUserMessage(params)
	ls.messages = append(ls.messages, userMsg)
	persistMessage(e, userMsg)

	for ls.turnCount < maxTurns {
		ls.turnCount++
		e.session.IncrTurn()

		// ── 5. Auto-compact at 80% context threshold ──────────────────────
		if e.cfg.MaxTokens > 0 && shouldCompact(ls.messages, e.cfg.MaxTokens) {
			slog.Info("queryloop: auto-compact triggered",
				slog.Int("turn", ls.turnCount),
				slog.String("session", e.cfg.SessionID))
			emitSystemMessage(out, "Compacting conversation to free context space…")
			compacted, _, err := CompactMessages(ctx, e.caller, ls.messages, e.cfg.Model)
			if err != nil {
				slog.Warn("queryloop: auto-compact failed", slog.Any("err", err))
			} else {
				ls.messages = compacted
			}
		}

		callParams := CallParams{
			Model:          e.cfg.Model,
			MaxTokens:      e.cfg.MaxTokens,
			ThinkingBudget: e.cfg.ThinkingBudget,
			SystemPrompt:   ls.systemPrompt,
			Messages:       ls.messages,
			Tools:          e.toolDefs(),
			UsePromptCache: true,
		}

		eventCh, err := e.caller.CallModel(ctx, callParams)
		if err != nil {
			return fmt.Errorf("callModel: %w", err)
		}

		// Consume the event stream from the provider.
		assistantMsg, toolCalls, stop, err := drainProviderStream(ctx, eventCh, out, e)
		if err != nil {
			return err
		}

		// ── 6. Persist and append assistant turn ──────────────────────────
		if assistantMsg != nil && len(assistantMsg.Content) > 0 {
			ls.messages = append(ls.messages, assistantMsg)
			persistMessage(e, assistantMsg)
		}

		if stop || len(toolCalls) == 0 {
			out <- &StreamEvent{Type: EventDone, SessionID: e.cfg.SessionID}
			return nil
		}

		// Execute tool calls and append results.
		toolResultMsg, err := executeToolCalls(ctx, e, toolCalls, out)
		if err != nil {
			return err
		}
		ls.messages = append(ls.messages, toolResultMsg)
		persistMessage(e, toolResultMsg)
	}

	return fmt.Errorf("exceeded maximum turn limit (%d)", maxTurns)
}

// shouldCompact reports whether the accumulated message tokens exceed the
// compactThreshold fraction of maxTokens.
func shouldCompact(messages []*Message, maxTokens int) bool {
	if maxTokens <= 0 {
		return false
	}
	tokenEst := EstimateTokens(messages)
	return float64(tokenEst) >= float64(maxTokens)*compactThreshold
}

// persistMessage writes a message to the session writer if one is configured.
func persistMessage(e *Engine, msg *Message) {
	if e.sessionWriter == nil {
		return
	}
	if err := e.sessionWriter.AppendMessage(e.cfg.SessionID, msg); err != nil {
		slog.Warn("queryloop: session persist failed", slog.Any("err", err))
	}
}

// buildSystemPromptIntegrated uses the injected SystemPromptBuilder when
// available, falling back to the lightweight stub.
func buildSystemPromptIntegrated(e *Engine, memoryContent string) string {
	if e.promptBuilder != nil {
		return e.promptBuilder.Build(SystemPromptOptions{
			Tools:              e.enabledTools(),
			UseContext:         e.useContext(),
			WorkDir:            e.cfg.WorkDir,
			MemoryContent:      memoryContent,
			CustomSystemPrompt: e.cfg.CustomSystemPrompt,
			AppendSystemPrompt: e.cfg.AppendSystemPrompt,
		})
	}
	return buildSystemPrompt(e)
}

// drainProviderStream reads events from the provider channel, forwards
// text/thinking/usage events to `out`, accumulates the assistant message,
// and returns any pending tool calls.
func drainProviderStream(
	ctx context.Context,
	eventCh <-chan *StreamEvent,
	out chan<- *StreamEvent,
	e *Engine,
) (*Message, []*pendingToolCall, bool, error) {

	assistantMsg := &Message{
		ID:   uuid.New().String(),
		Role: RoleAssistant,
	}

	var toolCalls []*pendingToolCall
	// map toolID -> accumulated input JSON
	toolInputBuf := make(map[string]*json.RawMessage)
	stop := false

	for {
		select {
		case <-ctx.Done():
			return nil, nil, false, ctx.Err()
		case ev, ok := <-eventCh:
			if !ok {
				return assistantMsg, toolCalls, stop, nil
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
				}
				out <- ev

			case EventError:
				return nil, nil, false, fmt.Errorf("provider error: %s", ev.Error)

			case EventDone:
				stop = true

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

// executeToolCalls runs all pending tool calls (concurrently where safe)
// and returns a user message containing all tool results.
func executeToolCalls(
	ctx context.Context,
	e *Engine,
	calls []*pendingToolCall,
	out chan<- *StreamEvent,
) (*Message, error) {

	resultMsg := &Message{
		ID:   uuid.New().String(),
		Role: RoleUser,
	}

	for _, tc := range calls {
		t, ok := e.findTool(tc.Name)
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

		// ── Global permission check (policy rules) ────────────────────────
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

		// ── Auto Mode LLM classifier ──────────────────────────────────────
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

		// ── Per-tool permission check ─────────────────────────────────────
		if err := t.CheckPermissions(ctx, tc.Input, uctx); err != nil {
			resultMsg.Content = append(resultMsg.Content, &ContentBlock{
				Type:      ContentTypeToolResult,
				ToolUseID: tc.ID,
				Content:   []*ContentBlock{{Type: ContentTypeText, Text: "Permission denied: " + err.Error()}},
				IsError:   true,
			})
			continue
		}

		// Execute
		blockCh, err := t.Call(ctx, tc.Input, uctx)
		if err != nil {
			resultMsg.Content = append(resultMsg.Content, &ContentBlock{
				Type:      ContentTypeToolResult,
				ToolUseID: tc.ID,
				Content:   []*ContentBlock{{Type: ContentTypeText, Text: err.Error()}},
				IsError:   true,
			})
			continue
		}

		// Collect all result blocks.
		var resultBlocks []*ContentBlock
		isErr := false
		for b := range blockCh {
			if b.IsError {
				isErr = true
			}
			resultBlocks = append(resultBlocks, b)
		}

		out <- &StreamEvent{
			Type:     EventToolResult,
			ToolID:   tc.ID,
			ToolName: tc.Name,
			IsError:  isErr,
		}

		resultMsg.Content = append(resultMsg.Content, &ContentBlock{
			Type:      ContentTypeToolResult,
			ToolUseID: tc.ID,
			Content:   resultBlocks,
			IsError:   isErr,
		})
	}

	return resultMsg, nil
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
