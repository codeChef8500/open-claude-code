package mode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/provider"
)

// RunYoloClassifier runs the LLM-based Auto Mode permission classifier.
// It uses a lightweight model (Haiku) as a side query without affecting
// the main conversation context.
func RunYoloClassifier(
	ctx context.Context,
	prov provider.Provider,
	toolName string,
	toolInput interface{},
	userRules []AutoModeRule,
) (ClassifierVerdict, string, error) {

	systemPrompt := buildYoloSystemPrompt(append(DefaultAutoModeRules, userRules...))

	inputJSON, err := json.MarshalIndent(toolInput, "", "  ")
	if err != nil {
		return VerdictDeny, "", err
	}

	userText := fmt.Sprintf("Tool: %s\nInput:\n%s\n\nShould this be allowed? Reply with JSON: {\"verdict\":\"allow\"|\"soft_deny\"|\"deny\",\"reason\":\"...\"}",
		toolName, string(inputJSON))

	messages := []*engine.Message{
		{
			Role:    engine.RoleUser,
			Content: []*engine.ContentBlock{{Type: engine.ContentTypeText, Text: userText}},
		},
	}

	params := provider.CallParams{
		Model:          "claude-haiku-4-5",
		MaxTokens:      256,
		SystemPrompt:   systemPrompt,
		Messages:       messages,
		UsePromptCache: true,
	}

	eventCh, err := prov.CallModel(ctx, params)
	if err != nil {
		return VerdictDeny, "", err
	}

	var responseText string
	for ev := range eventCh {
		if ev.Type == engine.EventTextDelta {
			responseText += ev.Text
		}
	}

	verdict, reason := parseClassifierResponse(responseText)
	return verdict, reason, nil
}

func buildYoloSystemPrompt(rules []AutoModeRule) string {
	var sb strings.Builder
	sb.WriteString("You are a permission classifier for an AI coding assistant in Auto Mode.\n\n")
	sb.WriteString("## Rules\n")
	for _, r := range rules {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", r.Type, r.Description))
	}
	sb.WriteString("\nClassify the proposed tool call and respond with JSON only.")
	return sb.String()
}

func parseClassifierResponse(text string) (ClassifierVerdict, string) {
	text = strings.TrimSpace(text)
	// Strip markdown code fences if present
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var resp struct {
		Verdict string `json:"verdict"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		// Default to deny on parse failure.
		return VerdictDeny, "classifier response parse error"
	}

	switch ClassifierVerdict(resp.Verdict) {
	case VerdictAllow:
		return VerdictAllow, resp.Reason
	case VerdictSoftDeny:
		return VerdictSoftDeny, resp.Reason
	default:
		return VerdictDeny, resp.Reason
	}
}
