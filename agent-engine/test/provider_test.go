package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/provider"
)

// TestMockProviderEmitsEvents re-uses the mockProvider declared in engine_test.go.
func TestMockProviderEmitsEvents(t *testing.T) {
	prov := &mockProvider{response: "Hello, world"}

	ctx := context.Background()
	ch, err := prov.CallModel(ctx, engine.CallParams{})
	require.NoError(t, err)

	var text string
	var gotDone bool
	for ev := range ch {
		switch ev.Type {
		case engine.EventTextDelta:
			text += ev.Text
		case engine.EventDone:
			gotDone = true
		}
	}

	assert.Equal(t, "Hello, world", text)
	assert.True(t, gotDone)
}

func TestProviderFactory(t *testing.T) {
	tests := []struct {
		name    string
		cfg     provider.Config
		wantErr bool
	}{
		{"empty type defaults to anthropic", provider.Config{Type: "", APIKey: "sk-test"}, false},
		{"explicit anthropic", provider.Config{Type: "anthropic", APIKey: "sk-test"}, false},
		{"openai compat", provider.Config{Type: "openai", APIKey: "sk-test"}, false},
		{"unknown type returns error", provider.Config{Type: "unknown_xyz"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := provider.New(tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCallParamsFields(t *testing.T) {
	params := engine.CallParams{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 1024,
	}
	assert.Equal(t, "claude-sonnet-4-5", params.Model)
	assert.Equal(t, 1024, params.MaxTokens)
	assert.Nil(t, params.Messages)
}
