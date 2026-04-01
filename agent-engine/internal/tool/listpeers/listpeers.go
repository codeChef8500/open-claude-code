package listpeers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// ListPeersTool lists all currently active peer agents in the session.
type ListPeersTool struct{}

func New() *ListPeersTool { return &ListPeersTool{} }

func (t *ListPeersTool) Name() string            { return "list_peers" }
func (t *ListPeersTool) UserFacingName() string  { return "ListPeers" }
func (t *ListPeersTool) Description() string {
	return "List all currently active peer agents and their statuses."
}
func (t *ListPeersTool) IsReadOnly() bool        { return true }
func (t *ListPeersTool) IsConcurrencySafe() bool { return true }
func (t *ListPeersTool) MaxResultSizeChars() int { return 4000 }
func (t *ListPeersTool) IsEnabled(_ *tool.UseContext) bool { return true }

func (t *ListPeersTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{}}`)
}

func (t *ListPeersTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *ListPeersTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil
}

func (t *ListPeersTool) Call(_ context.Context, _ json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	ch := make(chan *engine.ContentBlock, 1)
	go func() {
		defer close(ch)

		var sb strings.Builder
		sb.WriteString("Active peer agents:\n")

		if uctx != nil && uctx.AgentID != "" {
			sb.WriteString(fmt.Sprintf("  - self: %s (current)\n", uctx.AgentID))
		} else {
			sb.WriteString("  (no active peers — running as primary agent)\n")
		}

		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: sb.String()}
	}()
	return ch, nil
}
