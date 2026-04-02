package engine

// ────────────────────────────────────────────────────────────────────────────
// SDK message types — the external-facing envelope that SDK callers and the
// HTTP SSE layer consume.  Aligned with claude-code-main's
// entrypoints/agentSdkTypes.ts SDKMessage union.
// ────────────────────────────────────────────────────────────────────────────

// SDKMessageType discriminates the different SDK envelope variants.
type SDKMessageType string

const (
	SDKMsgAssistant        SDKMessageType = "assistant"
	SDKMsgUser             SDKMessageType = "user"
	SDKMsgSystem           SDKMessageType = "system"
	SDKMsgResultStatus     SDKMessageType = "result_status"
	SDKMsgCompactBoundary  SDKMessageType = "compact_boundary"
	SDKMsgPermissionDenial SDKMessageType = "permission_denial"
	SDKMsgProgress         SDKMessageType = "progress"
	SDKMsgToolUseSummary   SDKMessageType = "tool_use_summary"
)

// SDKMessage is the top-level envelope emitted by the engine for external
// consumers (SDK, HTTP SSE, bridge, etc.).
type SDKMessage struct {
	Type SDKMessageType `json:"type"`

	// ── assistant ───────────────────────────────────────────────────────
	Message *SDKAssistantMessage `json:"message,omitempty"`

	// ── result_status ──────────────────────────────────────────────────
	ResultStatus *SDKResultStatus `json:"result_status,omitempty"`

	// ── compact_boundary ───────────────────────────────────────────────
	CompactBoundary *SDKCompactBoundary `json:"compact_boundary,omitempty"`

	// ── permission_denial ──────────────────────────────────────────────
	PermissionDenial *SDKPermissionDenial `json:"permission_denial,omitempty"`

	// ── system ─────────────────────────────────────────────────────────
	SystemText  string             `json:"system_text,omitempty"`
	SystemLevel SystemMessageLevel `json:"system_level,omitempty"`

	// ── progress ───────────────────────────────────────────────────────
	ProgressData *ProgressData `json:"progress_data,omitempty"`

	// ── tool_use_summary ───────────────────────────────────────────────
	ToolUseSummaryData *ToolUseSummaryData `json:"tool_use_summary_data,omitempty"`

	// Shared fields.
	SessionID string `json:"session_id,omitempty"`
}

// SDKAssistantMessage is the SDK representation of an assistant turn.
type SDKAssistantMessage struct {
	UUID       string          `json:"uuid"`
	Role       string          `json:"role"` // always "assistant"
	Content    []*ContentBlock `json:"content"`
	Model      string          `json:"model,omitempty"`
	StopReason string          `json:"stop_reason,omitempty"`
	Usage      *SDKUsage       `json:"usage,omitempty"`
	CostUSD    float64         `json:"cost_usd,omitempty"`
}

// SDKAssistantMessageError wraps an API error in the assistant message slot.
type SDKAssistantMessageError struct {
	UUID     string `json:"uuid"`
	Role     string `json:"role"` // "assistant"
	APIError string `json:"api_error"`
	Error    string `json:"error"`
}

// SDKUsage is the external-facing usage summary.
type SDKUsage struct {
	InputTokens              int     `json:"input_tokens"`
	OutputTokens             int     `json:"output_tokens"`
	CacheCreationInputTokens int     `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int     `json:"cache_read_input_tokens,omitempty"`
	CostUSD                  float64 `json:"cost_usd,omitempty"`
	ServerDurationMs         int     `json:"server_duration_ms,omitempty"`
}

// SDKResultStatus carries the final status of a query invocation.
type SDKResultStatus struct {
	IsError      bool    `json:"is_error"`
	Duration     int     `json:"duration_ms"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	TurnCount    int     `json:"turn_count"`
	Message      string  `json:"message,omitempty"` // human-readable summary
}

// SDKCompactBoundary marks where compaction occurred.
type SDKCompactBoundary struct {
	TokensFreed     int    `json:"tokens_freed,omitempty"`
	TokensRemaining int    `json:"tokens_remaining,omitempty"`
	Direction       string `json:"direction,omitempty"`
}

// SDKPermissionDenial represents a permission denial event.
type SDKPermissionDenial struct {
	ToolName string `json:"tool_name"`
	Reason   string `json:"reason"`
	RuleType string `json:"rule_type,omitempty"` // "auto_classifier", "rule_based", "hook"
}

// ────────────────────────────────────────────────────────────────────────────
// Conversion helpers: internal Message → SDKMessage
// ────────────────────────────────────────────────────────────────────────────

// ToSDKUsage converts internal UsageStats to the external SDKUsage type.
func ToSDKUsage(u *UsageStats) *SDKUsage {
	if u == nil {
		return nil
	}
	return &SDKUsage{
		InputTokens:              u.InputTokens,
		OutputTokens:             u.OutputTokens,
		CacheCreationInputTokens: u.CacheCreationInputTokens,
		CacheReadInputTokens:     u.CacheReadInputTokens,
		CostUSD:                  u.CostUSD,
		ServerDurationMs:         u.ServerDurationMs,
	}
}

// MessageToSDK converts an internal Message to an SDKMessage.
// Returns nil for message types that have no SDK representation (e.g. meta messages).
func MessageToSDK(m *Message, sessionID string) *SDKMessage {
	switch m.Type {
	case MsgTypeAssistant:
		return &SDKMessage{
			Type:      SDKMsgAssistant,
			SessionID: sessionID,
			Message: &SDKAssistantMessage{
				UUID:       m.UUID,
				Role:       string(RoleAssistant),
				Content:    m.Content,
				Model:      m.Model,
				StopReason: m.StopReason,
				Usage:      ToSDKUsage(m.Usage),
				CostUSD:    m.CostUSD,
			},
		}

	case MsgTypeSystemAPIError:
		return &SDKMessage{
			Type:        SDKMsgSystem,
			SessionID:   sessionID,
			SystemText:  m.Error,
			SystemLevel: SystemLevelError,
		}

	case MsgTypeSystemInformational, MsgTypeSystemLocalCommand,
		MsgTypeSystemMemorySaved, MsgTypeSystemPermissionRetry:
		text := ""
		if len(m.Content) > 0 {
			text = m.Content[0].Text
		}
		return &SDKMessage{
			Type:        SDKMsgSystem,
			SessionID:   sessionID,
			SystemText:  text,
			SystemLevel: m.Level,
		}

	case MsgTypeCompactBoundary:
		return &SDKMessage{
			Type:      SDKMsgCompactBoundary,
			SessionID: sessionID,
			CompactBoundary: &SDKCompactBoundary{
				Direction: "forward",
			},
		}

	case MsgTypeProgress:
		return &SDKMessage{
			Type:         SDKMsgProgress,
			SessionID:    sessionID,
			ProgressData: m.ProgressData,
		}

	case MsgTypeToolUseSummary:
		return &SDKMessage{
			Type:               SDKMsgToolUseSummary,
			SessionID:          sessionID,
			ToolUseSummaryData: m.ToolUseSummary,
		}

	default:
		// User messages, tombstones, etc. — no SDK representation.
		return nil
	}
}

// StreamEventToSDK converts a StreamEvent to an SDKMessage for external consumption.
func StreamEventToSDK(ev *StreamEvent, sessionID string) *SDKMessage {
	switch ev.Type {
	case EventTextComplete:
		return &SDKMessage{
			Type:      SDKMsgAssistant,
			SessionID: sessionID,
			Message: &SDKAssistantMessage{
				UUID:    ev.MessageUUID,
				Role:    string(RoleAssistant),
				Content: []*ContentBlock{{Type: ContentTypeText, Text: ev.Text}},
				Usage:   ToSDKUsage(ev.Usage),
			},
		}

	case EventError:
		return &SDKMessage{
			Type:        SDKMsgSystem,
			SessionID:   sessionID,
			SystemText:  ev.Error,
			SystemLevel: SystemLevelError,
		}

	case EventSystemMessage:
		return &SDKMessage{
			Type:        SDKMsgSystem,
			SessionID:   sessionID,
			SystemText:  ev.Text,
			SystemLevel: SystemMessageLevel(ev.Level),
		}

	case EventCompactBoundary:
		sdk := &SDKCompactBoundary{}
		if ev.CompactInfo != nil {
			sdk.TokensFreed = ev.CompactInfo.TokensFreed
			sdk.TokensRemaining = ev.CompactInfo.TokensRemaining
			sdk.Direction = ev.CompactInfo.Direction
		}
		return &SDKMessage{
			Type:            SDKMsgCompactBoundary,
			SessionID:       sessionID,
			CompactBoundary: sdk,
		}

	default:
		return nil
	}
}
