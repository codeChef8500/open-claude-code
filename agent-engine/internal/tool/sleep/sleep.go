package sleep

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/state"
	"github.com/wall-ai/agent-engine/internal/tool"
)

type Input struct {
	Milliseconds int `json:"milliseconds"`
}

type SleepTool struct{ tool.BaseTool }

func New() *SleepTool { return &SleepTool{} }

func (t *SleepTool) Name() string                             { return "Sleep" }
func (t *SleepTool) UserFacingName() string                   { return "sleep" }
func (t *SleepTool) Description() string                      { return "Sleep for the specified number of milliseconds." }
func (t *SleepTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (t *SleepTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *SleepTool) MaxResultSizeChars() int                  { return 0 }
func (t *SleepTool) IsEnabled(_ *tool.UseContext) bool        { return true }
func (t *SleepTool) InterruptBehavior() engine.InterruptBehavior {
	return engine.InterruptBehaviorStop
}

func (t *SleepTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"milliseconds":{"type":"integer","description":"Duration to sleep in milliseconds."}
		},
		"required":["milliseconds"]
	}`)
}

func (t *SleepTool) Prompt(_ *tool.UseContext) string {
	return `Sleep for the specified number of milliseconds. Use this tool instead of Bash(sleep ...) when you need to wait.

In assistant daemon mode, the system sends <tick> prompts periodically. Prefer
Sleep over idle loops — sleeping keeps the prompt cache alive (caches expire
after ~5 minutes of inactivity). For daemon workers, longer sleeps are allowed.

Maximum sleep duration: 300000ms (5 min) in daemon mode, 60000ms (1 min) otherwise.`
}

func (t *SleepTool) CheckPermissions(_ context.Context, input json.RawMessage, uctx *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Milliseconds < 0 {
		return fmt.Errorf("milliseconds must be non-negative")
	}
	// Daemon workers allow longer sleep (5 min); interactive caps at 60s
	maxMs := 60_000
	if uctx != nil && uctx.GetAppState != nil {
		if v := uctx.GetAppState(); v != nil {
			if as, ok := v.(*state.AppState); ok && state.IsDaemonSession(as.SessionKind) {
				maxMs = 300_000
			}
		}
	}
	if in.Milliseconds > maxMs {
		return fmt.Errorf("sleep duration exceeds maximum (%dms)", maxMs)
	}
	return nil
}

func (t *SleepTool) Call(ctx context.Context, input json.RawMessage, _ *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)
		select {
		case <-time.After(time.Duration(in.Milliseconds) * time.Millisecond):
			ch <- &engine.ContentBlock{
				Type: engine.ContentTypeText,
				Text: fmt.Sprintf("Slept for %dms", in.Milliseconds),
			}
		case <-ctx.Done():
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "Sleep cancelled", IsError: true}
		}
	}()
	return ch, nil
}
