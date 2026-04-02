package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// SSELine represents a single server-sent event field.
type SSELine struct {
	Field string
	Value string
}

// ParseSSELine parses a single "field: value" SSE line.
// Returns empty SSELine for comment lines (#) or blank lines.
func ParseSSELine(line string) SSELine {
	line = strings.TrimRight(line, "\r")
	if line == "" || strings.HasPrefix(line, ":") {
		return SSELine{}
	}
	idx := strings.IndexByte(line, ':')
	if idx < 0 {
		return SSELine{Field: line}
	}
	field := line[:idx]
	value := line[idx+1:]
	value = strings.TrimPrefix(value, " ")
	return SSELine{Field: field, Value: value}
}

// StreamReader reads an SSE stream and emits parsed events to a channel.
// The channel is closed when the stream ends or ctx is cancelled.
func StreamReader(ctx context.Context, r io.Reader) <-chan map[string]string {
	out := make(chan map[string]string, 32)
	go func() {
		defer close(out)
		buf := make([]byte, 32*1024)
		var pending strings.Builder
		event := make(map[string]string)

		emit := func() {
			if len(event) == 0 {
				return
			}
			select {
			case <-ctx.Done():
			case out <- event:
			}
			event = make(map[string]string)
		}

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, err := r.Read(buf)
			if n > 0 {
				pending.Write(buf[:n])
				for {
					text := pending.String()
					idx := strings.IndexByte(text, '\n')
					if idx < 0 {
						break
					}
					line := text[:idx]
					pending.Reset()
					pending.WriteString(text[idx+1:])
					if line == "" {
						emit()
						continue
					}
					sl := ParseSSELine(line)
					if sl.Field != "" {
						event[sl.Field] = sl.Value
					}
				}
			}
			if err != nil {
				emit()
				return
			}
		}
	}()
	return out
}

// DrainStreamEvents reads all events from ch and converts them to engine.StreamEvents.
// Anthropic SSE format: event field = type, data field = JSON payload.
func DrainStreamEvents(ch <-chan map[string]string) []*engine.StreamEvent {
	var events []*engine.StreamEvent
	for raw := range ch {
		ev := sseToStreamEvent(raw)
		if ev != nil {
			events = append(events, ev)
		}
	}
	return events
}

func sseToStreamEvent(raw map[string]string) *engine.StreamEvent {
	data, ok := raw["data"]
	if !ok || data == "[DONE]" {
		return nil
	}
	evType := raw["event"]
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return &engine.StreamEvent{
			Type:  engine.EventError,
			Error: fmt.Sprintf("sse parse error: %v", err),
		}
	}
	switch evType {
	case "content_block_delta":
		var delta struct {
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &delta); err == nil && delta.Delta.Text != "" {
			return &engine.StreamEvent{
				Type: engine.EventTextDelta,
				Text: delta.Delta.Text,
			}
		}
	case "message_delta":
		var md struct {
			Usage *engine.UsageStats `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &md); err == nil && md.Usage != nil {
			return &engine.StreamEvent{
				Type:  engine.EventUsage,
				Usage: md.Usage,
			}
		}
	case "message_stop":
		return &engine.StreamEvent{Type: engine.EventDone}
	case "error":
		var er struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &er); err == nil {
			return &engine.StreamEvent{
				Type:  engine.EventError,
				Error: er.Error.Message,
			}
		}
	}
	return nil
}
