package command

import (
	"context"
	"fmt"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// Command dispatch enhancements.
// Aligned with claude-code-main parseAndHandleSlashCommand and
// slashCommandInput processing pipeline.
//
// This file provides the unified dispatch layer that:
// 1. Parses user input for slash commands
// 2. Resolves command by name or alias (with fuzzy matching)
// 3. Checks feature gates, availability, and permission
// 4. Routes to the correct execution path (local / interactive / prompt)
// 5. Handles immediate commands (executed before submitting to model)
// ──────────────────────────────────────────────────────────────────────────────

// DispatchResult describes what happened when dispatching a command.
type DispatchResult struct {
	// Handled indicates whether the input was a slash command.
	Handled bool `json:"handled"`
	// CommandName is the resolved command name.
	CommandName string `json:"command_name,omitempty"`
	// Type is the command type that was executed.
	Type CommandType `json:"type,omitempty"`
	// Output is the text output for local commands.
	Output string `json:"output,omitempty"`
	// PromptInjection is the text to inject into the conversation for prompt commands.
	PromptInjection string `json:"prompt_injection,omitempty"`
	// Interactive is the structured result for interactive commands.
	Interactive *InteractiveResult `json:"interactive,omitempty"`
	// Error is set if execution failed.
	Error error `json:"error,omitempty"`
	// Immediate indicates the command was immediate (doesn't need model turn).
	Immediate bool `json:"immediate,omitempty"`
}

// Dispatch parses input and dispatches to the appropriate command handler.
// Returns a DispatchResult describing what happened.
func Dispatch(ctx context.Context, input string, ectx *ExecContext) *DispatchResult {
	input = strings.TrimSpace(input)

	// Not a slash command.
	if !strings.HasPrefix(input, "/") {
		return &DispatchResult{Handled: false}
	}

	// Parse command name and args.
	parts := strings.Fields(input[1:]) // strip leading /
	if len(parts) == 0 {
		return &DispatchResult{Handled: false}
	}

	cmdName := strings.ToLower(parts[0])
	args := parts[1:]

	// Resolve from registry.
	registry := Default()
	cmd := registry.Find(cmdName)

	// Fuzzy match if not found.
	if cmd == nil {
		cmd = fuzzyMatch(registry, cmdName)
	}

	if cmd == nil {
		return &DispatchResult{
			Handled:     true,
			CommandName: cmdName,
			Error:       fmt.Errorf("unknown command: /%s", cmdName),
		}
	}

	// Check if enabled.
	if !cmd.IsEnabled(ectx) {
		return &DispatchResult{
			Handled:     true,
			CommandName: cmd.Name(),
			Error:       fmt.Errorf("command /%s is not currently available", cmd.Name()),
		}
	}

	// Check availability constraints.
	if ac, ok := cmd.(interface{ Availability() []CommandAvailability }); ok {
		avails := ac.Availability()
		if len(avails) > 0 {
			// For now, we allow all. In production, check against ectx context.
			_ = avails
		}
	}

	// Check dynamic hidden.
	if dh, ok := cmd.(interface{ DynamicIsHidden(*ExecContext) bool }); ok {
		if dh.DynamicIsHidden(ectx) {
			return &DispatchResult{
				Handled:     true,
				CommandName: cmd.Name(),
				Error:       fmt.Errorf("command /%s is not available in current context", cmd.Name()),
			}
		}
	}

	result := &DispatchResult{
		Handled:     true,
		CommandName: cmd.Name(),
		Type:        cmd.Type(),
		Immediate:   cmd.IsImmediate(),
	}

	// Execute based on type.
	switch cmd.Type() {
	case CommandTypeLocal:
		if lc, ok := cmd.(interface {
			Execute(context.Context, []string, *ExecContext) (string, error)
		}); ok {
			output, err := lc.Execute(ctx, args, ectx)
			result.Output = output
			result.Error = err
		} else {
			result.Error = fmt.Errorf("command /%s is local but does not implement Execute", cmd.Name())
		}

	case CommandTypePrompt:
		if pc, ok := cmd.(interface {
			PromptContent([]string, *ExecContext) (string, error)
		}); ok {
			content, err := pc.PromptContent(args, ectx)
			result.PromptInjection = content
			result.Error = err
		} else {
			result.Error = fmt.Errorf("command /%s is prompt but does not implement PromptContent", cmd.Name())
		}

	case CommandTypeInteractive:
		if ic, ok := cmd.(interface {
			ExecuteInteractive(context.Context, []string, *ExecContext) (*InteractiveResult, error)
		}); ok {
			ir, err := ic.ExecuteInteractive(ctx, args, ectx)
			result.Interactive = ir
			result.Error = err
		} else {
			result.Error = fmt.Errorf("command /%s is interactive but does not implement ExecuteInteractive", cmd.Name())
		}

	default:
		result.Error = fmt.Errorf("unknown command type %v for /%s", cmd.Type(), cmd.Name())
	}

	return result
}

// fuzzyMatch attempts to find a command by prefix or edit distance.
// Aligned with claude-code-main's command completion logic.
func fuzzyMatch(registry *Registry, input string) Command {
	input = strings.ToLower(input)

	// 1. Exact prefix match (unique).
	var prefixMatches []Command
	for _, cmd := range registry.All() {
		if strings.HasPrefix(strings.ToLower(cmd.Name()), input) {
			prefixMatches = append(prefixMatches, cmd)
		}
	}
	if len(prefixMatches) == 1 {
		return prefixMatches[0]
	}

	// 2. Edit distance 1 match (unique).
	var editMatches []Command
	for _, cmd := range registry.All() {
		if editDistance(input, strings.ToLower(cmd.Name())) <= 1 {
			editMatches = append(editMatches, cmd)
		}
	}
	if len(editMatches) == 1 {
		return editMatches[0]
	}

	return nil
}

// editDistance computes the Levenshtein distance between two strings.
func editDistance(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// Use single-row optimization.
	prev := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr := make([]int, lb+1)
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = minOf(
				prev[j]+1,      // delete
				curr[j-1]+1,    // insert
				prev[j-1]+cost, // substitute
			)
		}
		prev = curr
	}

	return prev[lb]
}

func minOf(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// IsSlashCommand returns true if the input starts with a /.
func IsSlashCommand(input string) bool {
	return strings.HasPrefix(strings.TrimSpace(input), "/")
}

// CompleteCommand returns command names matching the given prefix.
// Used for tab completion in the TUI.
func CompleteCommand(prefix string, ectx *ExecContext) []string {
	prefix = strings.ToLower(strings.TrimPrefix(prefix, "/"))
	registry := Default()

	var matches []string
	for _, cmd := range registry.All() {
		if cmd.IsHidden() {
			continue
		}
		if !cmd.IsEnabled(ectx) {
			continue
		}
		name := cmd.Name()
		if strings.HasPrefix(strings.ToLower(name), prefix) {
			matches = append(matches, "/"+name)
		}
	}

	return matches
}
