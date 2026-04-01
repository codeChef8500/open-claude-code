package test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/service"
)

func TestEstimateTokensBasic(t *testing.T) {
	// 4 chars ≈ 1 token
	assert.Equal(t, 1, service.EstimateTokens("abcd"))
	assert.Equal(t, 2, service.EstimateTokens("abcdefgh"))
	assert.Equal(t, 0, service.EstimateTokens(""))
	// Short strings still return at least 1
	assert.Equal(t, 1, service.EstimateTokens("hi"))
}

func TestEstimateMessagesTokens(t *testing.T) {
	msgs := []*engine.Message{
		{
			Role: engine.RoleUser,
			Content: []*engine.ContentBlock{
				{Type: engine.ContentTypeText, Text: "Hello world, this is a test message."},
			},
		},
		{
			Role: engine.RoleAssistant,
			Content: []*engine.ContentBlock{
				{Type: engine.ContentTypeText, Text: "I understand your request."},
			},
		},
	}
	tokens := service.EstimateMessagesTokens(msgs)
	assert.Greater(t, tokens, 0)
	// Rough sanity: total chars / 4 + overhead
	assert.Greater(t, tokens, 5)
}

func TestExceedsThreshold(t *testing.T) {
	msgs := []*engine.Message{
		{
			Role: engine.RoleUser,
			Content: []*engine.ContentBlock{
				{Type: engine.ContentTypeText, Text: "aaaa"}, // ~1 token
			},
		},
	}

	// maxTokens=0 should never trigger
	assert.False(t, service.ExceedsThreshold(msgs, 0, 0.8))
	// With very high maxTokens, threshold should not be exceeded
	assert.False(t, service.ExceedsThreshold(msgs, 1_000_000, 0.8))
	// With very low maxTokens (1), even 1 estimated token hits 80%
	assert.True(t, service.ExceedsThreshold(msgs, 1, 0.8))
}

func TestTruncateToTokenBudget(t *testing.T) {
	var msgs []*engine.Message
	for i := 0; i < 20; i++ {
		msgs = append(msgs, &engine.Message{
			Role: engine.RoleUser,
			Content: []*engine.ContentBlock{
				// 40 chars = ~10 tokens per message
				{Type: engine.ContentTypeText, Text: "this is a filler message for test run ok"},
			},
		})
	}

	budget := 50 // tokens
	trimmed := service.TruncateToTokenBudget(msgs, budget)
	assert.LessOrEqual(t, service.EstimateMessagesTokens(trimmed), budget+20) // allow small overshoot
	assert.Less(t, len(trimmed), len(msgs))
}
