package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	openai "github.com/sashabaranov/go-openai"
	"github.com/wall-ai/agent-engine/internal/engine"
)

// OpenAICompatProvider implements Provider for OpenAI-compatible APIs
// (OpenAI, Ollama, vLLM, LM Studio, etc.).
type OpenAICompatProvider struct {
	client *openai.Client
	model  string
}

// NewOpenAICompatProvider creates a provider for OpenAI-compatible endpoints.
func NewOpenAICompatProvider(apiKey, model, baseURL string) *OpenAICompatProvider {
	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	if model == "" {
		model = "gpt-4o"
	}
	return &OpenAICompatProvider{
		client: openai.NewClientWithConfig(cfg),
		model:  model,
	}
}

func (p *OpenAICompatProvider) Name() string { return "openai" }

func (p *OpenAICompatProvider) CallModel(ctx context.Context, params CallParams) (<-chan *engine.StreamEvent, error) {
	ch := make(chan *engine.StreamEvent, 64)
	go func() {
		defer close(ch)
		if err := p.stream(ctx, params, ch); err != nil {
			ch <- &engine.StreamEvent{Type: engine.EventError, Error: err.Error()}
		}
	}()
	return ch, nil
}

func (p *OpenAICompatProvider) stream(ctx context.Context, params CallParams, ch chan<- *engine.StreamEvent) error {
	model := params.Model
	if model == "" {
		model = p.model
	}
	maxTokens := params.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 8192
	}

	// Build messages
	var messages []openai.ChatCompletionMessage
	if params.SystemPrompt != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: params.SystemPrompt,
		})
	}
	for _, m := range params.Messages {
		converted, err := convertMessageToOpenAI(m)
		if err != nil {
			return err
		}
		messages = append(messages, converted...)
	}

	// Build tools
	var tools []openai.Tool
	for _, t := range params.Tools {
		schemaBytes, _ := json.Marshal(t.InputSchema)
		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  json.RawMessage(schemaBytes),
			},
		})
	}

	req := openai.ChatCompletionRequest{
		Model:     model,
		MaxTokens: maxTokens,
		Messages:  messages,
		Stream:    true,
	}
	if len(tools) > 0 {
		req.Tools = tools
	}

	stream, err := p.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return fmt.Errorf("openai stream create: %w", err)
	}
	defer stream.Close()

	// Track tool call accumulation
	toolCallBuf := make(map[int]*toolCallAccum)

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("openai stream recv: %w", err)
		}
		if len(resp.Choices) == 0 {
			continue
		}
		delta := resp.Choices[0].Delta

		// Text delta
		if delta.Content != "" {
			ch <- &engine.StreamEvent{Type: engine.EventTextDelta, Text: delta.Content}
		}

		// Tool call deltas
		for _, tc := range delta.ToolCalls {
			idx := tc.Index
			if idx == nil {
				continue
			}
			i := *idx
			if toolCallBuf[i] == nil {
				toolCallBuf[i] = &toolCallAccum{}
			}
			if tc.ID != "" {
				toolCallBuf[i].ID = tc.ID
			}
			if tc.Function.Name != "" {
				toolCallBuf[i].Name = tc.Function.Name
			}
			toolCallBuf[i].ArgsJSON += tc.Function.Arguments
		}

		// Finish reason
		if resp.Choices[0].FinishReason == openai.FinishReasonToolCalls {
			for _, accum := range toolCallBuf {
				var input interface{}
				_ = json.Unmarshal([]byte(accum.ArgsJSON), &input)
				ch <- &engine.StreamEvent{
					Type:      engine.EventToolUse,
					ToolID:    accum.ID,
					ToolName:  accum.Name,
					ToolInput: input,
				}
			}
		}
	}

	// Usage stats not easily available in streaming mode for OpenAI compat.
	ch <- &engine.StreamEvent{Type: engine.EventDone}
	return nil
}

type toolCallAccum struct {
	ID       string
	Name     string
	ArgsJSON string
}

func convertMessageToOpenAI(m *engine.Message) ([]openai.ChatCompletionMessage, error) {
	var result []openai.ChatCompletionMessage

	switch m.Role {
	case engine.RoleUser, engine.RoleAssistant:
		role := openai.ChatMessageRoleUser
		if m.Role == engine.RoleAssistant {
			role = openai.ChatMessageRoleAssistant
		}

		var textParts []openai.ChatMessagePart
		var toolCalls []openai.ToolCall

		for _, b := range m.Content {
			switch b.Type {
			case engine.ContentTypeText:
				textParts = append(textParts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeText,
					Text: b.Text,
				})
			case engine.ContentTypeToolUse:
				argsBytes, _ := json.Marshal(b.Input)
				toolCalls = append(toolCalls, openai.ToolCall{
					ID:   b.ToolUseID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      b.ToolName,
						Arguments: string(argsBytes),
					},
				})
			case engine.ContentTypeToolResult:
				// Tool results go as separate "tool" role messages
				var content string
				for _, inner := range b.Content {
					content += inner.Text
				}
				result = append(result, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    content,
					ToolCallID: b.ToolUseID,
				})
			}
		}

		msg := openai.ChatCompletionMessage{Role: role}
		if len(toolCalls) > 0 {
			msg.ToolCalls = toolCalls
		} else if len(textParts) == 1 {
			msg.Content = textParts[0].Text
		} else if len(textParts) > 1 {
			msg.MultiContent = textParts
		}
		if msg.Content != "" || len(msg.ToolCalls) > 0 || len(msg.MultiContent) > 0 {
			result = append([]openai.ChatCompletionMessage{msg}, result...)
		}
	}
	return result, nil
}
