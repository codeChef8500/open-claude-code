package command

import (
	"context"
	"fmt"
	"strings"
)

// Executor dispatches slash commands entered by the user.
type Executor struct {
	registry *Registry
}

// NewExecutor creates an Executor backed by the given registry.
func NewExecutor(r *Registry) *Executor {
	if r == nil {
		r = Default()
	}
	return &Executor{registry: r}
}

// Execute parses a raw slash-command string (e.g. "/compact foo bar") and
// runs it.  It returns the result string and any execution error.
// If the input does not start with "/" it returns an empty string and nil.
func (e *Executor) Execute(ctx context.Context, raw string, ectx *ExecContext) (string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "/") {
		return "", nil
	}

	// Strip the leading "/" and split into name + args.
	rest := raw[1:]
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return "", nil
	}

	name := strings.ToLower(parts[0])
	args := parts[1:]

	cmd := e.registry.Find(name)
	if cmd == nil {
		return "", fmt.Errorf("unknown command: /%s (type /help for a list)", name)
	}
	if !cmd.IsEnabled(ectx) {
		return "", fmt.Errorf("command /%s is not available in the current context", name)
	}

	return dispatchCommand(ctx, cmd, name, args, ectx)
}

// Execute is a package-level convenience function that dispatches a command
// by name (without leading slash) using the default registry.
func Execute(ctx context.Context, name string, args []string, ectx *ExecContext) (string, error) {
	cmd := Default().Find(name)
	if cmd == nil {
		return "", fmt.Errorf("unknown command: /%s (type /help for a list)", name)
	}
	if !cmd.IsEnabled(ectx) {
		return "", fmt.Errorf("command /%s is not available in the current context", name)
	}
	return dispatchCommand(ctx, cmd, name, args, ectx)
}

func dispatchCommand(ctx context.Context, cmd Command, name string, args []string, ectx *ExecContext) (string, error) {
	switch c := cmd.(type) {
	case InteractiveCommand:
		// InteractiveCommand returns structured data for TUI rendering.
		result, err := c.ExecuteInteractive(ctx, args, ectx)
		if err != nil {
			return "", err
		}
		if result == nil {
			return "", nil
		}
		// Extract fallback text from the result data so text-mode TUI can
		// display meaningful output instead of a generic "/<cmd> executed." stub.
		if fb := extractFallbackText(result.Data); fb != "" {
			return "__interactive__:" + result.Component + "\n" + fb, nil
		}
		// Encode as __interactive__:<component> for the TUI layer to handle.
		return "__interactive__:" + result.Component, nil
	case LocalCommand:
		return c.Execute(ctx, args, ectx)
	case PromptCommand:
		content, err := c.PromptContent(args, ectx)
		if err != nil {
			return "", err
		}
		// Check if the command should run as a forked sub-agent.
		if meta := c.PromptMeta(); meta != nil && meta.ExecContext == "fork" {
			return "__fork_prompt__:" + content, nil
		}
		return "__prompt__:" + content, nil
	default:
		return "", fmt.Errorf("command /%s has unknown type", name)
	}
}

// extractFallbackText attempts to pull a FallbackText string from an
// InteractiveResult.Data value.  Many interactive commands store a
// FallbackText field on their view-data struct for non-interactive contexts.
func extractFallbackText(data interface{}) string {
	if data == nil {
		return ""
	}
	// Check for the FallbackTexter interface first.
	if ft, ok := data.(interface{ GetFallbackText() string }); ok {
		return ft.GetFallbackText()
	}
	// Fall back to reflection-free duck typing on common concrete types.
	switch d := data.(type) {
	case *HelpViewData:
		return d.FallbackText
	case *StatusViewDataV2:
		return d.FallbackText
	case *SessionViewData:
		return d.FallbackText
	default:
		// Use a map-based probe for arbitrary data (e.g. map[string]interface{}).
		if m, ok := d.(map[string]interface{}); ok {
			if fb, ok := m["fallback_text"].(string); ok {
				return fb
			}
		}
		return ""
	}
}

// IsCommand reports whether raw looks like a slash command.
func IsCommand(raw string) bool {
	return strings.HasPrefix(strings.TrimSpace(raw), "/")
}
