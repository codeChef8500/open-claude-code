package engine

// ────────────────────────────────────────────────────────────────────────────
// Query loop state transitions — aligned with claude-code-main query.ts
// Terminal/Continue types that drive the while(true) state machine.
// ────────────────────────────────────────────────────────────────────────────

// TerminalReason enumerates why the query loop exited.
type TerminalReason string

const (
	TerminalCompleted           TerminalReason = "completed"
	TerminalAbortedStreaming    TerminalReason = "aborted_streaming"
	TerminalAbortedTools        TerminalReason = "aborted_tools"
	TerminalBlockingLimit       TerminalReason = "blocking_limit"
	TerminalModelError          TerminalReason = "model_error"
	TerminalImageError          TerminalReason = "image_error"
	TerminalPromptTooLong       TerminalReason = "prompt_too_long"
	TerminalStopHookPrevented   TerminalReason = "stop_hook_prevented"
	TerminalHookStopped         TerminalReason = "hook_stopped"
	TerminalMaxTurns            TerminalReason = "max_turns"
)

// Terminal is the final result of a queryLoop execution.
type Terminal struct {
	// Reason is the high-level reason the loop exited.
	Reason TerminalReason
	// Error is the underlying error, if any (for model_error, image_error).
	Error error
	// TurnCount is the number of turns completed before exit.
	TurnCount int
}

// ContinueReason enumerates why the loop is continuing to the next iteration.
type ContinueReason string

const (
	ContinueNextTurn               ContinueReason = "next_turn"
	ContinueMaxOutputTokensRecovery ContinueReason = "max_output_tokens_recovery"
	ContinueMaxOutputTokensEscalate ContinueReason = "max_output_tokens_escalate"
	ContinueReactiveCompactRetry   ContinueReason = "reactive_compact_retry"
	ContinueCollapseDrainRetry     ContinueReason = "collapse_drain_retry"
	ContinueStopHookBlocking       ContinueReason = "stop_hook_blocking"
	ContinueTokenBudgetContinuation ContinueReason = "token_budget_continuation"
)

// ContinueTransition records the reason and metadata for the current iteration's
// continue decision. Stored in loopState.transition.
type ContinueTransition struct {
	// Reason is the high-level reason for continuing.
	Reason ContinueReason
	// Committed is the number of context-collapse entries committed (for collapse_drain_retry).
	Committed int
	// Attempt is the recovery attempt number (for max_output_tokens_recovery).
	Attempt int
}

// ────────────────────────────────────────────────────────────────────────────
// Extended QuerySource constants — aligned with claude-code-main querySource.ts
// Note: base QuerySource type and some constants already exist in types.go.
// These extend the set to match the full TS enum.
// ────────────────────────────────────────────────────────────────────────────

const (
	QuerySourceSDK             QuerySource = "sdk"
	QuerySourceREPLMainThread  QuerySource = "repl_main_thread"
	QuerySourceREPLSideThread  QuerySource = "repl_side_thread"
)

// IsCompactOrSessionMemory returns true if the source is a background
// compaction or session-memory query that should skip certain safety checks
// (e.g. token blocking limit, reactive compact gating).
func (qs QuerySource) IsCompactOrSessionMemory() bool {
	return qs == QuerySourceCompact || qs == QuerySourceSessionMemory
}

// IsMainThread returns true if the source originates from the main user thread
// (REPL main or SDK), as opposed to subagents or background forks.
func (qs QuerySource) IsMainThread() bool {
	return qs == QuerySourceSDK || qs == QuerySourceREPLMainThread
}
