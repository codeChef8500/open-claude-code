package provider

import (
	"testing"
)

func TestResolveModel_ExactMatch(t *testing.T) {
	spec := ResolveModel("claude-sonnet-4")
	if spec.Family != ModelFamilyClaude {
		t.Errorf("expected claude family, got %s", spec.Family)
	}
	if spec.ContextWindow != 200_000 {
		t.Errorf("expected 200k context, got %d", spec.ContextWindow)
	}
	if !spec.SupportsThinking {
		t.Error("claude-sonnet-4 should support thinking")
	}
}

func TestResolveModel_PrefixMatch(t *testing.T) {
	spec := ResolveModel("claude-sonnet-4-20250514")
	if spec.Family != ModelFamilyClaude {
		t.Errorf("expected claude family, got %s", spec.Family)
	}
	if spec.Name != "claude-sonnet-4-20250514" {
		t.Errorf("expected caller's exact name preserved, got %q", spec.Name)
	}
}

func TestResolveModel_GPT(t *testing.T) {
	spec := ResolveModel("gpt-4o")
	if spec.Family != ModelFamilyGPT {
		t.Errorf("expected gpt family, got %s", spec.Family)
	}
	if spec.ContextWindow != 128_000 {
		t.Errorf("expected 128k context, got %d", spec.ContextWindow)
	}
}

func TestResolveModel_Unknown(t *testing.T) {
	spec := ResolveModel("some-custom-model")
	if spec.Family != ModelFamilyUnknown {
		t.Errorf("expected unknown family, got %s", spec.Family)
	}
	if spec.ContextWindow != 200_000 {
		t.Errorf("expected fallback 200k context, got %d", spec.ContextWindow)
	}
}

func TestResolveModel_HeuristicClaude(t *testing.T) {
	spec := ResolveModel("claude-future-model-99")
	if spec.Family != ModelFamilyClaude {
		t.Errorf("expected claude family via heuristic, got %s", spec.Family)
	}
}

func TestResolveModel_HeuristicGemini(t *testing.T) {
	spec := ResolveModel("gemini-pro-2")
	if spec.Family != ModelFamilyGemini {
		t.Errorf("expected gemini family, got %s", spec.Family)
	}
}

func TestIsClaude(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"claude-sonnet-4", true},
		{"Claude-Opus-4", true},
		{"gpt-4o", false},
		{"gemini-pro", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsClaude(tt.model)
		if got != tt.want {
			t.Errorf("IsClaude(%q) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestIsThinkingModel(t *testing.T) {
	if !IsThinkingModel("claude-sonnet-4") {
		t.Error("claude-sonnet-4 should be a thinking model")
	}
	if IsThinkingModel("claude-3-5-sonnet") {
		t.Error("claude-3-5-sonnet should not be a thinking model")
	}
	if IsThinkingModel("gpt-4o") {
		t.Error("gpt-4o should not be a thinking model")
	}
}

func TestContextWindowFor(t *testing.T) {
	if cw := ContextWindowFor("claude-sonnet-4"); cw != 200_000 {
		t.Errorf("expected 200000, got %d", cw)
	}
	if cw := ContextWindowFor("gpt-4o"); cw != 128_000 {
		t.Errorf("expected 128000, got %d", cw)
	}
	// Unknown model should get safe default.
	if cw := ContextWindowFor("unknown-model"); cw != 200_000 {
		t.Errorf("expected fallback 200000, got %d", cw)
	}
}

func TestResolveModel_Opus(t *testing.T) {
	spec := ResolveModel("claude-opus-4")
	if !spec.SupportsThinking {
		t.Error("opus should support thinking")
	}
	if spec.MaxOutputTokens != 32_000 {
		t.Errorf("expected 32000 max output, got %d", spec.MaxOutputTokens)
	}
}

func TestResolveModel_Haiku(t *testing.T) {
	spec := ResolveModel("claude-haiku-4-5")
	if spec.SupportsThinking {
		t.Error("haiku should not support thinking")
	}
	if spec.Family != ModelFamilyClaude {
		t.Errorf("expected claude family, got %s", spec.Family)
	}
}
