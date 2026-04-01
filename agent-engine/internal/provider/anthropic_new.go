package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/wall-ai/agent-engine/internal/engine"
)

const (
	maxRetries    = 3
	retryBaseMs   = 500
)

// AnthropicProvider implements Provider (engine.ModelCaller) via the official
// Anthropic Go SDK v0.2.0-beta.3. It wraps the synchronous Messages.New call
// in a goroutine to produce the streaming event channel our interface requires.
type AnthropicProvider struct {
	client anthropic.Client // value type — NewClient returns by value
	model  string
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(apiKey, model, baseURL string) *AnthropicProvider {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := anthropic.NewClient(opts...)
	if model == "" {
		model = "claude-sonnet-4-5"
	}
	return &AnthropicProvider{client: client, model: model}
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

func (p *AnthropicProvider) CallModel(ctx context.Context, params CallParams) (<-chan *engine.StreamEvent, error) {
	ch := make(chan *engine.StreamEvent, 64)
	go func() {
		defer close(ch)
		var lastErr error
		for attempt := 0; attempt < maxRetries; attempt++ {
			if attempt > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(retryBaseMs*(1<<uint(attempt-1))) * time.Millisecond):
				}
			}
			lastErr = p.call(ctx, params, ch)
			if lastErr == nil {
				return
			}
			errStr := lastErr.Error()
			if !strings.Contains(errStr, "429") && !strings.Contains(errStr, "529") && !strings.Contains(errStr, "overloaded") {
				break
			}
		}
		if lastErr != nil {
			ch <- &engine.StreamEvent{Type: engine.EventError, Error: lastErr.Error()}
		}
	}()
	return ch, nil
}

func (p *AnthropicProvider) call(ctx context.Context, params CallParams, ch chan<- *engine.StreamEvent) error {
	model := params.Model
	if model == "" {
		model = p.model
	}
	maxTokens := params.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 8192
	}

	apiMessages, err := convertMessagesToAnthropic(params.Messages)
	if err != nil {
		return fmt.Errorf("convert messages: %w", err)
	}

	var apiTools []anthropic.ToolUnionParam
	for _, t := range params.Tools {
		schemaMap := toSchemaMap(t.InputSchema)
		desc := t.Description
		apiTools = append(apiTools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(desc),
				InputSchema: anthropic.ToolInputSchemaParam{Properties: schemaMap},
			},
		})
	}

	req := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(maxTokens),
		Messages:  apiMessages,
	}

	// Build system prompt: prefer multi-segment parts for cache-aware injection;
	// fall back to single-block SystemPrompt when parts are not provided.
	// Anthropic supports up to 4 cache_control blocks; we cache Layer 1 (base prompt)
	// and Layer 2 (tool descriptions) — the two most stable segments.
	if len(params.SystemPromptParts) > 0 {
		var sysBlocks []anthropic.TextBlockParam
		for _, part := range params.SystemPromptParts {
			if part.Content == "" {
				continue
			}
			block := anthropic.TextBlockParam{Text: part.Content}
			if params.UsePromptCache && !params.SkipCacheWrite && part.CacheHint {
				block.CacheControl = anthropic.CacheControlEphemeralParam{}
			}
			sysBlocks = append(sysBlocks, block)
		}
		req.System = sysBlocks
	} else if params.SystemPrompt != "" {
		block := anthropic.TextBlockParam{Text: params.SystemPrompt}
		if params.UsePromptCache && !params.SkipCacheWrite {
			block.CacheControl = anthropic.CacheControlEphemeralParam{}
		}
		req.System = []anthropic.TextBlockParam{block}
	}

	if len(apiTools) > 0 {
		req.Tools = apiTools
	}
	if params.ThinkingBudget > 0 {
		req.Thinking = anthropic.ThinkingConfigParamOfThinkingConfigEnabled(int64(params.ThinkingBudget))
	}

	// Use the streaming API for real-time token delivery.
	stream := p.client.Messages.NewStreaming(ctx, req)
	defer stream.Close()

	// Track tool-use blocks being assembled during streaming.
	type streamingTool struct {
		id    string
		name  string
		input strings.Builder
	}
	toolsByIndex := make(map[int]*streamingTool)

	// Accumulate the full message for final usage stats.
	var accMsg anthropic.Message

	for stream.Next() {
		ev := stream.Current()
		_ = accMsg.Accumulate(ev)

		switch ev.Type {
		case "content_block_start":
			idx := int(ev.Index)
			if ev.ContentBlock.Type == "tool_use" {
				toolsByIndex[idx] = &streamingTool{id: ev.ContentBlock.ID, name: ev.ContentBlock.Name}
			}

		case "content_block_delta":
			idx := int(ev.Index)
			switch ev.Delta.Type {
			case "text_delta":
				ch <- &engine.StreamEvent{Type: engine.EventTextDelta, Text: ev.Delta.Text}
			case "thinking_delta":
				ch <- &engine.StreamEvent{Type: engine.EventThinking, Thinking: ev.Delta.Thinking}
			case "input_json_delta":
				if tc, ok := toolsByIndex[idx]; ok {
					tc.input.WriteString(ev.Delta.PartialJSON)
				}
			}

		case "content_block_stop":
			idx := int(ev.Index)
			if tc, ok := toolsByIndex[idx]; ok {
				inputRaw := tc.input.String()
				if inputRaw == "" {
					inputRaw = "{}"
				}
				var inputMap interface{}
				_ = json.Unmarshal([]byte(inputRaw), &inputMap)
				ch <- &engine.StreamEvent{
					Type:      engine.EventToolUse,
					ToolID:    tc.id,
					ToolName:  tc.name,
					ToolInput: inputMap,
				}
				delete(toolsByIndex, idx)
			}

		case "message_delta":
			if ev.Usage.OutputTokens > 0 {
				ch <- &engine.StreamEvent{
					Type: engine.EventUsage,
					Usage: &engine.UsageStats{
						InputTokens:              int(accMsg.Usage.InputTokens),
						OutputTokens:             int(accMsg.Usage.OutputTokens),
						CacheCreationInputTokens: int(accMsg.Usage.CacheCreationInputTokens),
						CacheReadInputTokens:     int(accMsg.Usage.CacheReadInputTokens),
					},
				}
			}
		}
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("anthropic stream: %w", err)
	}

	ch <- &engine.StreamEvent{Type: engine.EventDone}
	return nil
}

// convertMessagesToAnthropic converts internal messages to Anthropic API format.
func convertMessagesToAnthropic(msgs []*engine.Message) ([]anthropic.MessageParam, error) {
	var result []anthropic.MessageParam
	for _, m := range msgs {
		blocks, err := convertContentBlocksToAnthropic(m.Content)
		if err != nil {
			return nil, err
		}
		switch m.Role {
		case engine.RoleUser:
			result = append(result, anthropic.NewUserMessage(blocks...))
		case engine.RoleAssistant:
			result = append(result, anthropic.NewAssistantMessage(blocks...))
		}
	}
	return result, nil
}

func convertContentBlocksToAnthropic(blocks []*engine.ContentBlock) ([]anthropic.ContentBlockParamUnion, error) {
	var result []anthropic.ContentBlockParamUnion
	for _, b := range blocks {
		switch b.Type {
		case engine.ContentTypeText:
			result = append(result, anthropic.NewTextBlock(b.Text))

		case engine.ContentTypeToolUse:
			var inputMap interface{}
			if raw, err := json.Marshal(b.Input); err == nil {
				_ = json.Unmarshal(raw, &inputMap)
			}
			result = append(result, anthropic.ContentBlockParamUnion{
				OfRequestToolUseBlock: &anthropic.ToolUseBlockParam{
					ID:    b.ToolUseID,
					Name:  b.ToolName,
					Input: inputMap,
				},
			})

		case engine.ContentTypeToolResult:
			// Combine inner text blocks into a single string.
			var parts []string
			for _, inner := range b.Content {
				if inner.Type == engine.ContentTypeText {
					parts = append(parts, inner.Text)
				}
			}
			combined := strings.Join(parts, "\n")
			// NewToolResultBlock takes (toolUseID, content, isError string).
			result = append(result, anthropic.NewToolResultBlock(b.ToolUseID, combined, b.IsError))

		case engine.ContentTypeThinking:
			result = append(result, anthropic.ContentBlockParamUnion{
				OfRequestThinkingBlock: &anthropic.ThinkingBlockParam{
					Thinking:  b.Thinking,
					Signature: b.Signature,
				},
			})

		case engine.ContentTypeImage:
			result = append(result, anthropic.NewImageBlockBase64(b.MediaType, b.Data))
		}
	}
	return result, nil
}

func toSchemaMap(schema interface{}) map[string]interface{} {
	if m, ok := schema.(map[string]interface{}); ok {
		return m
	}
	b, err := json.Marshal(schema)
	if err != nil {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	return m
}
