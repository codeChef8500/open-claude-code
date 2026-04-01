package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/wall-ai/agent-engine/internal/engine"
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
		if err := p.call(ctx, params, ch); err != nil {
			ch <- &engine.StreamEvent{Type: engine.EventError, Error: err.Error()}
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
	if params.SystemPrompt != "" {
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
		// Helper function avoids direct struct literal field assignment.
		req.Thinking = anthropic.ThinkingConfigParamOfThinkingConfigEnabled(int64(params.ThinkingBudget))
	}

	msg, err := p.client.Messages.New(ctx, req)
	if err != nil {
		return fmt.Errorf("anthropic api: %w", err)
	}

	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			ch <- &engine.StreamEvent{Type: engine.EventTextDelta, Text: block.Text}
		case "thinking":
			ch <- &engine.StreamEvent{Type: engine.EventThinking, Thinking: block.Thinking}
		case "tool_use":
			// block.Input is json.RawMessage in the response type
			var inputMap interface{}
			_ = json.Unmarshal(block.Input, &inputMap)
			ch <- &engine.StreamEvent{
				Type:      engine.EventToolUse,
				ToolID:    block.ID,
				ToolName:  block.Name,
				ToolInput: inputMap,
			}
		}
	}

	ch <- &engine.StreamEvent{
		Type: engine.EventUsage,
		Usage: &engine.UsageStats{
			InputTokens:              int(msg.Usage.InputTokens),
			OutputTokens:             int(msg.Usage.OutputTokens),
			CacheCreationInputTokens: int(msg.Usage.CacheCreationInputTokens),
			CacheReadInputTokens:     int(msg.Usage.CacheReadInputTokens),
		},
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
