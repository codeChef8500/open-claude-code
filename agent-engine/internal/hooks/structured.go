package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ────────────────────────────────────────────────────────────────────────────
// Structured output enforcement — hooks that validate and enforce output
// format constraints on tool results and assistant messages.
// Aligned with claude-code-main's registerStructuredOutputEnforcement.
// ────────────────────────────────────────────────────────────────────────────

// OutputFormat specifies the expected output format for structured output enforcement.
type OutputFormat string

const (
	OutputFormatJSON     OutputFormat = "json"
	OutputFormatXML      OutputFormat = "xml"
	OutputFormatMarkdown OutputFormat = "markdown"
	OutputFormatPlain    OutputFormat = "plain"
)

// StructuredOutputConfig configures structured output enforcement.
type StructuredOutputConfig struct {
	// Format is the required output format.
	Format OutputFormat `json:"format"`

	// Schema is an optional JSON schema to validate against (for JSON format).
	Schema json.RawMessage `json:"schema,omitempty"`

	// RequiredFields are field names that must be present in JSON output.
	RequiredFields []string `json:"required_fields,omitempty"`

	// MaxLength is the maximum allowed output length in characters (0 = unlimited).
	MaxLength int `json:"max_length,omitempty"`

	// StripExtraneous removes content outside the expected format markers.
	StripExtraneous bool `json:"strip_extraneous,omitempty"`
}

// RegisterStructuredOutputEnforcement registers a PostToolUse hook that validates
// tool output conforms to the specified format.
func RegisterStructuredOutputEnforcement(reg *Registry, toolName string, cfg StructuredOutputConfig) {
	name := fmt.Sprintf("structured_output_%s_%s", toolName, cfg.Format)

	reg.Register(EventPostToolUse, name, func(ctx context.Context, input *HookInput) (*HookJSONOutput, error) {
		if input.PostToolUse == nil || input.PostToolUse.ToolName != toolName {
			return nil, nil // not our tool
		}

		output := input.PostToolUse.Output
		if output == "" {
			return nil, nil
		}

		// Validate and potentially transform the output.
		validated, err := validateStructuredOutput(output, cfg)
		if err != nil {
			// Return the validation error as additional context.
			return &HookJSONOutput{
				AdditionalContext: fmt.Sprintf("Output format violation for %s: %s", toolName, err.Error()),
			}, nil
		}

		// If output was transformed, override it.
		if validated != output {
			return &HookJSONOutput{
				OutputOverride: &validated,
			}, nil
		}

		return nil, nil
	})
}

// RegisterOutputLengthEnforcement registers a PostToolUse hook that enforces
// maximum output length for a given tool.
func RegisterOutputLengthEnforcement(reg *Registry, toolName string, maxLength int) {
	name := fmt.Sprintf("output_length_%s", toolName)

	reg.Register(EventPostToolUse, name, func(ctx context.Context, input *HookInput) (*HookJSONOutput, error) {
		if input.PostToolUse == nil || input.PostToolUse.ToolName != toolName {
			return nil, nil
		}

		output := input.PostToolUse.Output
		if len(output) <= maxLength {
			return nil, nil
		}

		truncated := output[:maxLength] + "\n... [truncated by output length enforcement]"
		return &HookJSONOutput{
			OutputOverride: &truncated,
		}, nil
	})
}

// RegisterStopGuard registers a Stop hook that validates the assistant's final
// output meets criteria before allowing the session to end.
type StopGuardConfig struct {
	// RequireNonEmpty prevents stopping with an empty response.
	RequireNonEmpty bool `json:"require_non_empty,omitempty"`

	// RequiredSubstrings are strings that must appear in the final output.
	RequiredSubstrings []string `json:"required_substrings,omitempty"`

	// ForbiddenSubstrings are strings that must NOT appear in the final output.
	ForbiddenSubstrings []string `json:"forbidden_substrings,omitempty"`

	// MaxRetries limits how many times the stop can be rejected (0 = unlimited).
	MaxRetries int `json:"max_retries,omitempty"`
}

// RegisterStopGuard registers a Stop hook that validates the final output.
func RegisterStopGuard(reg *Registry, cfg StopGuardConfig) {
	retries := 0

	reg.Register(EventStop, "stop_guard", func(ctx context.Context, input *HookInput) (*HookJSONOutput, error) {
		if input.Stop == nil {
			return nil, nil
		}

		msg := input.Stop.AssistantMessage

		// Check retry limit.
		if cfg.MaxRetries > 0 && retries >= cfg.MaxRetries {
			passed := true
			return &HookJSONOutput{Passed: &passed}, nil
		}

		// Validate non-empty.
		if cfg.RequireNonEmpty && strings.TrimSpace(msg) == "" {
			retries++
			passed := false
			return &HookJSONOutput{
				Passed:        &passed,
				FailureReason: "response must not be empty",
			}, nil
		}

		// Validate required substrings.
		for _, req := range cfg.RequiredSubstrings {
			if !strings.Contains(msg, req) {
				retries++
				passed := false
				return &HookJSONOutput{
					Passed:        &passed,
					FailureReason: fmt.Sprintf("response must contain %q", req),
				}, nil
			}
		}

		// Validate forbidden substrings.
		for _, forbidden := range cfg.ForbiddenSubstrings {
			if strings.Contains(msg, forbidden) {
				retries++
				passed := false
				return &HookJSONOutput{
					Passed:        &passed,
					FailureReason: fmt.Sprintf("response must not contain %q", forbidden),
				}, nil
			}
		}

		passed := true
		return &HookJSONOutput{Passed: &passed}, nil
	})
}

// ── Validation helpers ──────────────────────────────────────────────────────

func validateStructuredOutput(output string, cfg StructuredOutputConfig) (string, error) {
	// Check max length.
	if cfg.MaxLength > 0 && len(output) > cfg.MaxLength {
		return "", fmt.Errorf("output exceeds maximum length (%d > %d)", len(output), cfg.MaxLength)
	}

	switch cfg.Format {
	case OutputFormatJSON:
		return validateJSONOutput(output, cfg)
	case OutputFormatXML:
		return validateXMLOutput(output, cfg)
	case OutputFormatMarkdown:
		return output, nil // markdown is always valid
	case OutputFormatPlain:
		return output, nil
	default:
		return output, nil
	}
}

func validateJSONOutput(output string, cfg StructuredOutputConfig) (string, error) {
	// Strip extraneous content (e.g. markdown code fences) if configured.
	cleaned := output
	if cfg.StripExtraneous {
		cleaned = stripJSONFences(cleaned)
	}

	// Validate JSON syntax.
	var parsed interface{}
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	// Check required fields.
	if len(cfg.RequiredFields) > 0 {
		if obj, ok := parsed.(map[string]interface{}); ok {
			for _, field := range cfg.RequiredFields {
				if _, exists := obj[field]; !exists {
					return "", fmt.Errorf("missing required field %q", field)
				}
			}
		}
	}

	return cleaned, nil
}

func validateXMLOutput(output string, cfg StructuredOutputConfig) (string, error) {
	cleaned := output
	if cfg.StripExtraneous {
		cleaned = stripXMLFences(cleaned)
	}

	// Basic XML validation: check for opening and closing tags.
	trimmed := strings.TrimSpace(cleaned)
	if !strings.HasPrefix(trimmed, "<") {
		return "", fmt.Errorf("output does not start with XML tag")
	}
	if !strings.HasSuffix(trimmed, ">") {
		return "", fmt.Errorf("output does not end with XML tag")
	}

	return cleaned, nil
}

func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}

func stripXMLFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```xml") {
		s = strings.TrimPrefix(s, "```xml")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}
