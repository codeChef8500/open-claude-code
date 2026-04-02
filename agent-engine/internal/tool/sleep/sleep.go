package sleep

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

type Input struct {
	Milliseconds int `json:"milliseconds"`
}

type SleepTool struct{ tool.BaseTool }

func New() *SleepTool { return &SleepTool{} }

func (t *SleepTool) Name() string                      { return "Sleep" }
func (t *SleepTool) UserFacingName() string            { return "sleep" }
func (t *SleepTool) Description() string               { return "Sleep for the specified number of milliseconds." }
func (t *SleepTool) IsReadOnly() bool                  { return true }
func (t *SleepTool) IsConcurrencySafe() bool           { return true }
func (t *SleepTool) MaxResultSizeChars() int           { return 0 }
func (t *SleepTool) IsEnabled(_ *tool.UseContext) bool { return true }
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

func (t *SleepTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *SleepTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Milliseconds < 0 {
		return fmt.Errorf("milliseconds must be non-negative")
	}
	if in.Milliseconds > 60_000 {
		return fmt.Errorf("sleep duration exceeds maximum (60000ms)")
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
