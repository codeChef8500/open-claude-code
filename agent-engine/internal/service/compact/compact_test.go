package compact

import (
	"testing"

	"github.com/wall-ai/agent-engine/internal/engine"
)

func makeMsg(role engine.MessageRole, text string) *engine.Message {
	return &engine.Message{
		Role:    role,
		Content: []*engine.ContentBlock{{Type: engine.ContentTypeText, Text: text}},
	}
}

func makeToolUseMsg() *engine.Message {
	return &engine.Message{
		Role: engine.RoleAssistant,
		Content: []*engine.ContentBlock{{
			Type:      engine.ContentTypeToolUse,
			ToolUseID: "t1",
			ToolName:  "Read",
			Text:      `{"path":"foo.go"}`,
		}},
	}
}

func makeToolResultMsg(toolID string) *engine.Message {
	return &engine.Message{
		Role: engine.RoleUser,
		Content: []*engine.ContentBlock{{
			Type:      engine.ContentTypeToolResult,
			ToolUseID: toolID,
			Text:      "file contents",
		}},
	}
}

func makeThinkingMsg() *engine.Message {
	return &engine.Message{
		Role: engine.RoleAssistant,
		Content: []*engine.ContentBlock{
			{Type: engine.ContentTypeThinking, Text: "thinking..."},
			{Type: engine.ContentTypeText, Text: "response"},
		},
	}
}

// ── Snip tests ──────────────────────────────────────────────────────────────

func TestSnip_NoOp(t *testing.T) {
	msgs := []*engine.Message{
		makeMsg(engine.RoleUser, "hi"),
		makeMsg(engine.RoleAssistant, "hello"),
	}
	result := Snip(msgs, SnipOptions{})
	if len(result) != 2 {
		t.Errorf("expected 2 (no-op), got %d", len(result))
	}
}

func TestSnip_DropMiddle(t *testing.T) {
	// 10 messages: keep first 2 + last 6 = 8, drop 2 from middle.
	msgs := make([]*engine.Message, 10)
	for i := range msgs {
		role := engine.RoleUser
		if i%2 == 1 {
			role = engine.RoleAssistant
		}
		msgs[i] = makeMsg(role, "msg")
	}

	result := Snip(msgs, SnipOptions{KeepFirstN: 2, KeepLastN: 6})
	if len(result) != 8 {
		t.Errorf("expected 8, got %d", len(result))
	}
}

func TestSnip_MaxMessagesToKeep(t *testing.T) {
	msgs := make([]*engine.Message, 20)
	for i := range msgs {
		msgs[i] = makeMsg(engine.RoleUser, "msg")
	}

	result := Snip(msgs, SnipOptions{MaxMessagesToKeep: 5, KeepFirstN: 2, KeepLastN: 3})
	if len(result) != 5 {
		t.Errorf("expected 5, got %d", len(result))
	}
}

func TestSnip_Empty(t *testing.T) {
	result := Snip(nil, SnipOptions{})
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

func TestStripThinkingBlocks(t *testing.T) {
	msgs := []*engine.Message{makeThinkingMsg()}
	result := stripThinkingBlocks(msgs)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	for _, b := range result[0].Content {
		if b.Type == engine.ContentTypeThinking {
			t.Error("thinking block should have been stripped")
		}
	}
}

func TestStripThinkingBlocks_AllThinking(t *testing.T) {
	msgs := []*engine.Message{{
		Role:    engine.RoleAssistant,
		Content: []*engine.ContentBlock{{Type: engine.ContentTypeThinking, Text: "only thinking"}},
	}}
	result := stripThinkingBlocks(msgs)
	if len(result) != 0 {
		t.Errorf("expected 0 messages when all content is thinking, got %d", len(result))
	}
}

// ── Grouping tests ──────────────────────────────────────────────────────────

func TestGroupByTurns_Simple(t *testing.T) {
	msgs := []*engine.Message{
		makeMsg(engine.RoleUser, "hello"),
		makeMsg(engine.RoleAssistant, "hi"),
		makeMsg(engine.RoleUser, "question"),
	}
	groups := GroupByTurns(msgs)
	if len(groups) != 3 {
		t.Errorf("expected 3 groups, got %d", len(groups))
	}
}

func TestGroupByTurns_WithToolResults(t *testing.T) {
	msgs := []*engine.Message{
		makeMsg(engine.RoleUser, "start"),
		makeToolUseMsg(),
		makeToolResultMsg("t1"),
		makeMsg(engine.RoleUser, "next"),
	}
	groups := GroupByTurns(msgs)
	// user(start), assistant+toolresult, user(next)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	if len(groups[1].ResultIdxs) != 1 {
		t.Errorf("expected 1 tool result in group 1, got %d", len(groups[1].ResultIdxs))
	}
}

func TestIsToolResultOnly(t *testing.T) {
	tr := makeToolResultMsg("t1")
	if !isToolResultOnly(tr) {
		t.Error("expected true for tool-result-only message")
	}

	mixed := &engine.Message{
		Role: engine.RoleUser,
		Content: []*engine.ContentBlock{
			{Type: engine.ContentTypeToolResult, ToolUseID: "t1"},
			{Type: engine.ContentTypeText, Text: "user text"},
		},
	}
	if isToolResultOnly(mixed) {
		t.Error("expected false for mixed content")
	}

	empty := &engine.Message{Role: engine.RoleUser}
	if isToolResultOnly(empty) {
		t.Error("expected false for empty content")
	}
}

func TestGroupMessagesByAPIRound(t *testing.T) {
	msgs := []*engine.Message{
		makeMsg(engine.RoleUser, "start"),
		makeToolUseMsg(),
		makeToolResultMsg("t1"),
		makeMsg(engine.RoleAssistant, "done"),
	}
	rounds := GroupMessagesByAPIRound(msgs)
	if len(rounds) != 3 {
		t.Errorf("expected 3 rounds, got %d", len(rounds))
	}
	// Round 0: user message
	// Round 1: assistant tool_use + tool_result
	// Round 2: assistant text
	if len(rounds[1].Messages) != 2 {
		t.Errorf("expected 2 messages in round 1, got %d", len(rounds[1].Messages))
	}
}

func TestGroupMessagesByAPIRound_Empty(t *testing.T) {
	rounds := GroupMessagesByAPIRound(nil)
	if rounds != nil {
		t.Error("expected nil for empty input")
	}
}

func TestTruncateHeadForPTLRetry(t *testing.T) {
	msgs := make([]*engine.Message, 12)
	for i := range msgs {
		role := engine.RoleUser
		if i%2 == 1 {
			role = engine.RoleAssistant
		}
		msgs[i] = makeMsg(role, "content that fills tokens")
	}

	result := TruncateHeadForPTLRetry(msgs, 0)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) >= len(msgs) {
		t.Errorf("expected fewer messages, got %d vs original %d", len(result), len(msgs))
	}
}

func TestTruncateHeadForPTLRetry_TooFew(t *testing.T) {
	msgs := []*engine.Message{makeMsg(engine.RoleUser, "only one")}
	result := TruncateHeadForPTLRetry(msgs, 100)
	if result != nil {
		t.Error("expected nil when too few messages to truncate")
	}
}

// ── SnipByGroups tests ──────────────────────────────────────────────────────

func TestSnipByGroups_PreservesToolPairs(t *testing.T) {
	msgs := make([]*engine.Message, 0)
	// Build 10 turn groups: user, assistant+tool, tool_result
	for i := 0; i < 10; i++ {
		msgs = append(msgs, makeMsg(engine.RoleUser, "q"))
		msgs = append(msgs, makeToolUseMsg())
		msgs = append(msgs, makeToolResultMsg("t1"))
	}

	result := SnipByGroups(msgs, SnipOptions{KeepFirstN: 2, KeepLastN: 3})
	// Should have dropped some middle groups but kept pairs intact.
	if len(result) >= len(msgs) {
		t.Errorf("expected fewer messages after snip, got %d", len(result))
	}
	// Verify no orphaned tool results.
	for i, m := range result {
		if m.Role == engine.RoleUser && isToolResultOnly(m) {
			if i == 0 {
				t.Error("tool result at start with no preceding assistant")
			}
		}
	}
}
