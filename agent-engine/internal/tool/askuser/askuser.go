package askuser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// ──────────────────────────────────────────────────────────────────────────────
// Constants — aligned with claude-code-main AskUserQuestionTool/prompt.ts
// ──────────────────────────────────────────────────────────────────────────────

const ToolName = "AskUserQuestion"

const chipWidth = 12

// ──────────────────────────────────────────────────────────────────────────────
// Schema types — aligned with claude-code-main questionSchema / questionOptionSchema
// ──────────────────────────────────────────────────────────────────────────────

// QuestionOption is a single selectable choice within a question.
type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	Preview     string `json:"preview,omitempty"`
}

// Question is one question block (1-4 per tool invocation).
type Question struct {
	QuestionText string           `json:"question"`
	Header       string           `json:"header"`
	Options      []QuestionOption `json:"options"`
	MultiSelect  bool             `json:"multiSelect,omitempty"`
}

// Annotation is optional per-question metadata collected from the user.
type Annotation struct {
	Preview string `json:"preview,omitempty"`
	Notes   string `json:"notes,omitempty"`
}

// Metadata carries analytics/tracking info.
type Metadata struct {
	Source string `json:"source,omitempty"`
}

// Input is the full tool input schema.
type Input struct {
	Questions   []Question            `json:"questions"`
	Answers     map[string]string     `json:"answers,omitempty"`
	Annotations map[string]Annotation `json:"annotations,omitempty"`
	Meta        *Metadata             `json:"metadata,omitempty"`
}

// Output is the structured tool output.
type Output struct {
	Questions   []Question            `json:"questions"`
	Answers     map[string]string     `json:"answers"`
	Annotations map[string]Annotation `json:"annotations,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Tool implementation
// ──────────────────────────────────────────────────────────────────────────────

type AskUserQuestionTool struct{ tool.BaseTool }

func New() *AskUserQuestionTool { return &AskUserQuestionTool{} }

func (t *AskUserQuestionTool) Name() string { return ToolName }

// UserFacingName returns "" — matching claude-code-main (no chip label).
func (t *AskUserQuestionTool) UserFacingName() string { return "" }

func (t *AskUserQuestionTool) Description() string {
	return "Asks the user multiple choice questions to gather information, clarify ambiguity, understand preferences, make decisions or offer them choices."
}

func (t *AskUserQuestionTool) SearchHint() string {
	return "prompt the user with a multiple-choice question"
}

// ── Feature flags ────────────────────────────────────────────────────────────

func (t *AskUserQuestionTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (t *AskUserQuestionTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *AskUserQuestionTool) ShouldDefer() bool                        { return true }
func (t *AskUserQuestionTool) RequiresUserInteraction() bool            { return true }
func (t *AskUserQuestionTool) MaxResultSizeChars() int                  { return 100_000 }

func (t *AskUserQuestionTool) IsEnabled(uctx *tool.UseContext) bool {
	// When KAIROS channels are active the user is not watching the TUI.
	// The multiple-choice dialog would hang with nobody at the keyboard.
	if uctx != nil && uctx.GetAppState != nil {
		if st := uctx.GetAppState(); st != nil {
			type channelHolder interface{ AllowedChannels() []string }
			if ch, ok := st.(channelHolder); ok && len(ch.AllowedChannels()) > 0 {
				return false
			}
		}
	}
	return true
}

// ToAutoClassifierInput returns a compact representation for classifiers.
func (t *AskUserQuestionTool) ToAutoClassifierInput(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return ""
	}
	parts := make([]string, 0, len(in.Questions))
	for _, q := range in.Questions {
		parts = append(parts, q.QuestionText)
	}
	return strings.Join(parts, " | ")
}

// ── Input schema ────────────────────────────────────────────────────────────

func (t *AskUserQuestionTool) InputSchema() json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{
	"type": "object",
	"properties": {
		"questions": {
			"type": "array",
			"minItems": 1,
			"maxItems": 4,
			"description": "Questions to ask the user (1-4 questions)",
			"items": {
				"type": "object",
				"properties": {
					"question": {
						"type": "string",
						"description": "The complete question to ask the user. Should be clear, specific, and end with a question mark."
					},
					"header": {
						"type": "string",
						"description": "Very short label displayed as a chip/tag (max %d chars). Examples: \"Auth method\", \"Library\", \"Approach\"."
					},
					"options": {
						"type": "array",
						"minItems": 2,
						"maxItems": 4,
						"description": "The available choices for this question. Must have 2-4 options. There should be no 'Other' option, that will be provided automatically.",
						"items": {
							"type": "object",
							"properties": {
								"label": {
									"type": "string",
									"description": "The display text for this option that the user will see and select. Should be concise (1-5 words)."
								},
								"description": {
									"type": "string",
									"description": "Explanation of what this option means or what will happen if chosen."
								},
								"preview": {
									"type": "string",
									"description": "Optional preview content rendered when this option is focused. Use for mockups, code snippets, or visual comparisons."
								}
							},
							"required": ["label", "description"]
						}
					},
					"multiSelect": {
						"type": "boolean",
						"default": false,
						"description": "Set to true to allow the user to select multiple options instead of just one."
					}
				},
				"required": ["question", "header", "options"]
			}
		},
		"answers": {
			"type": "object",
			"additionalProperties": { "type": "string" },
			"description": "User answers collected by the permission component"
		},
		"annotations": {
			"type": "object",
			"additionalProperties": {
				"type": "object",
				"properties": {
					"preview": { "type": "string", "description": "The preview content of the selected option." },
					"notes":   { "type": "string", "description": "Free-text notes the user added." }
				}
			},
			"description": "Optional per-question annotations from the user."
		},
		"metadata": {
			"type": "object",
			"properties": {
				"source": { "type": "string", "description": "Optional identifier for the source of this question." }
			},
			"description": "Optional metadata for tracking and analytics purposes."
		}
	},
	"required": ["questions"],
	"additionalProperties": false
}`, chipWidth))
}

// ── Output schema ───────────────────────────────────────────────────────────

func (t *AskUserQuestionTool) OutputSchema() json.RawMessage {
	return json.RawMessage(`{
	"type": "object",
	"properties": {
		"questions": { "type": "array", "items": { "type": "object" }, "description": "The questions that were asked" },
		"answers":   { "type": "object", "additionalProperties": { "type": "string" }, "description": "The answers provided by the user" },
		"annotations": { "type": "object", "description": "Optional per-question annotations" }
	},
	"required": ["questions", "answers"]
}`)
}

// ── Prompt ───────────────────────────────────────────────────────────────────

const toolPrompt = `Use this tool when you need to ask the user questions during execution. This allows you to:
1. Gather user preferences or requirements
2. Clarify ambiguous instructions
3. Get decisions on implementation choices as you work
4. Offer choices to the user about what direction to take.

Usage notes:
- Users will always be able to select "Other" to provide custom text input
- Use multiSelect: true to allow multiple answers to be selected for a question
- If you recommend a specific option, make that the first option in the list and add "(Recommended)" at the end of the label

Plan mode note: In plan mode, use this tool to clarify requirements or choose between approaches BEFORE finalizing your plan. Do NOT use this tool to ask "Is my plan ready?" or "Should I proceed?" - use ExitPlanMode for plan approval. IMPORTANT: Do not reference "the plan" in your questions (e.g., "Do you have feedback about the plan?", "Does the plan look good?") because the user cannot see the plan in the UI until you call ExitPlanMode. If you need plan approval, use ExitPlanMode instead.`

const previewPromptMarkdown = `

Preview feature:
Use the optional ` + "`preview`" + ` field on options when presenting concrete artifacts that users need to visually compare:
- ASCII mockups of UI layouts or components
- Code snippets showing different implementations
- Diagram variations
- Configuration examples

Preview content is rendered as markdown in a monospace box. Multi-line text with newlines is supported. When any option has a preview, the UI switches to a side-by-side layout with a vertical option list on the left and preview on the right. Do not use previews for simple preference questions where labels and descriptions suffice. Note: previews are only supported for single-select questions (not multiSelect).`

func (t *AskUserQuestionTool) Prompt(uctx *tool.UseContext) string {
	// Always include the markdown preview prompt for CLI mode.
	return toolPrompt + previewPromptMarkdown
}

// ── Validation ──────────────────────────────────────────────────────────────

func (t *AskUserQuestionTool) ValidateInput(_ context.Context, input json.RawMessage) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if len(in.Questions) == 0 {
		return fmt.Errorf("at least one question is required")
	}
	if len(in.Questions) > 4 {
		return fmt.Errorf("at most 4 questions are allowed")
	}

	// Uniqueness: question texts and headers must be unique.
	seenQ := make(map[string]struct{}, len(in.Questions))
	seenH := make(map[string]struct{}, len(in.Questions))
	for _, q := range in.Questions {
		if q.QuestionText == "" {
			return fmt.Errorf("question text must not be empty")
		}
		if _, dup := seenQ[q.QuestionText]; dup {
			return fmt.Errorf("duplicate question text: %q", q.QuestionText)
		}
		seenQ[q.QuestionText] = struct{}{}

		if q.Header != "" {
			if len([]rune(q.Header)) > chipWidth {
				return fmt.Errorf("header %q exceeds max %d chars", q.Header, chipWidth)
			}
			if _, dup := seenH[q.Header]; dup {
				return fmt.Errorf("duplicate header: %q", q.Header)
			}
			seenH[q.Header] = struct{}{}
		}

		if len(q.Options) < 2 {
			return fmt.Errorf("question %q must have at least 2 options", q.QuestionText)
		}
		if len(q.Options) > 4 {
			return fmt.Errorf("question %q must have at most 4 options", q.QuestionText)
		}

		// Uniqueness: labels within a question must be unique.
		seenL := make(map[string]struct{}, len(q.Options))
		for _, opt := range q.Options {
			if opt.Label == "" {
				return fmt.Errorf("option label must not be empty in question %q", q.QuestionText)
			}
			if _, dup := seenL[opt.Label]; dup {
				return fmt.Errorf("duplicate option label %q in question %q", opt.Label, q.QuestionText)
			}
			seenL[opt.Label] = struct{}{}
		}
	}
	return nil
}

// ── Permissions ─────────────────────────────────────────────────────────────

func (t *AskUserQuestionTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	// Always "ask" — the interactive permission component handles the dialog.
	return nil
}

// ── Call ─────────────────────────────────────────────────────────────────────

func (t *AskUserQuestionTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)

		// Build a summary of all questions for display.
		summary := buildSummary(in.Questions)

		// Use RequestPrompt if available (interactive prompt elicitation).
		if uctx.RequestPrompt != nil {
			promptFn := uctx.RequestPrompt(t.Name(), summary)
			if promptFn != nil {
				resp, err := promptFn(map[string]interface{}{
					"questions":   in.Questions,
					"answers":     in.Answers,
					"annotations": in.Annotations,
					"metadata":    in.Meta,
				})
				if err != nil {
					ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
					return
				}
				// The response should be an Output or a map.
				switch v := resp.(type) {
				case string:
					ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: v}
				case map[string]interface{}:
					data, _ := json.Marshal(v)
					ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(data)}
				default:
					data, _ := json.Marshal(v)
					ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(data)}
				}
				return
			}
		}

		// Fallback: if AskPermission callback, use it.
		if uctx.AskPermission != nil {
			approved, err := uctx.AskPermission(ctx, t.Name(), summary)
			if err != nil {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
				return
			}
			answer := "no"
			if approved {
				answer = "yes"
			}
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: answer}
			return
		}

		// No interactive channel — return a placeholder response.
		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: "[awaiting user response to: " + summary + "]",
		}
	}()
	return ch, nil
}

// ── MapToolResultToBlockParam ────────────────────────────────────────────────

func (t *AskUserQuestionTool) MapToolResultToBlockParam(content interface{}, toolUseID string) *engine.ContentBlock {
	// Try to extract structured output.
	var answers map[string]string
	var annotations map[string]Annotation

	switch v := content.(type) {
	case string:
		// Try to unmarshal as Output.
		var out Output
		if err := json.Unmarshal([]byte(v), &out); err == nil {
			answers = out.Answers
			annotations = out.Annotations
		} else {
			// Plain text answer.
			return &engine.ContentBlock{
				Type:      engine.ContentTypeToolResult,
				ToolUseID: toolUseID,
				Text:      "User has answered your questions: " + v + ". You can now continue with the user's answers in mind.",
			}
		}
	case map[string]interface{}:
		data, _ := json.Marshal(v)
		var out Output
		if err := json.Unmarshal(data, &out); err == nil {
			answers = out.Answers
			annotations = out.Annotations
		}
	}

	if len(answers) == 0 {
		return &engine.ContentBlock{
			Type:      engine.ContentTypeToolResult,
			ToolUseID: toolUseID,
			Text:      "User declined to answer questions.",
		}
	}

	// Format: "Q"="A" [selected preview:\n...] [user notes: ...], ...
	parts := make([]string, 0, len(answers))
	for qText, answer := range answers {
		p := fmt.Sprintf("%q=%q", qText, answer)
		if ann, ok := annotations[qText]; ok {
			if ann.Preview != "" {
				p += " selected preview:\n" + ann.Preview
			}
			if ann.Notes != "" {
				p += " user notes: " + ann.Notes
			}
		}
		parts = append(parts, p)
	}

	text := "User has answered your questions: " + strings.Join(parts, ", ") + ". You can now continue with the user's answers in mind."
	return &engine.ContentBlock{
		Type:      engine.ContentTypeToolResult,
		ToolUseID: toolUseID,
		Text:      text,
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func buildSummary(questions []Question) string {
	parts := make([]string, 0, len(questions))
	for _, q := range questions {
		s := q.QuestionText
		if len(q.Options) > 0 {
			labels := make([]string, 0, len(q.Options))
			for _, opt := range q.Options {
				labels = append(labels, opt.Label)
			}
			s += " [" + strings.Join(labels, ", ") + "]"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " | ")
}
