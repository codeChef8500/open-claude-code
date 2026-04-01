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

	switch c := cmd.(type) {
	case LocalCommand:
		return c.Execute(ctx, args, ectx)
	case PromptCommand:
		content, err := c.PromptContent(args, ectx)
		if err != nil {
			return "", err
		}
		return "__prompt__:" + content, nil
	default:
		return "", fmt.Errorf("command /%s has unknown type", name)
	}
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
	switch c := cmd.(type) {
	case LocalCommand:
		return c.Execute(ctx, args, ectx)
	case PromptCommand:
		content, err := c.PromptContent(args, ectx)
		if err != nil {
			return "", err
		}
		return "__prompt__:" + content, nil
	default:
		return "", fmt.Errorf("command /%s has unknown type", name)
	}
}

// IsCommand reports whether raw looks like a slash command.
func IsCommand(raw string) bool {
	return strings.HasPrefix(strings.TrimSpace(raw), "/")
}
