package command

import "strings"

// ParsedSlashCommand is the result of parsing a raw slash command string.
// Aligned with claude-code-main utils/slashCommandParsing.ts.
type ParsedSlashCommand struct {
	// CommandName is the command identifier without the leading "/".
	CommandName string
	// Args is the raw argument string after the command name.
	Args string
	// IsMCP is true when the command was invoked with the (MCP) suffix.
	IsMCP bool
}

// ParseSlashCommand parses a raw input string into its slash command parts.
// Returns nil if the input is not a valid slash command.
//
// Examples:
//
//	"/compact foo bar"       → {CommandName: "compact", Args: "foo bar"}
//	"/tool (MCP) some args"  → {CommandName: "tool (MCP)", Args: "some args", IsMCP: true}
//	"not a command"          → nil
func ParseSlashCommand(input string) *ParsedSlashCommand {
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, "/") {
		return nil
	}

	withoutSlash := trimmed[1:]
	words := strings.Fields(withoutSlash)
	if len(words) == 0 {
		return nil
	}

	commandName := words[0]
	isMCP := false
	argsStartIndex := 1

	// Check for "(MCP)" suffix: /name (MCP) args...
	if len(words) > 1 && words[1] == "(MCP)" {
		commandName = commandName + " (MCP)"
		isMCP = true
		argsStartIndex = 2
	}

	args := ""
	if argsStartIndex < len(words) {
		args = strings.Join(words[argsStartIndex:], " ")
	}

	return &ParsedSlashCommand{
		CommandName: commandName,
		Args:        args,
		IsMCP:       isMCP,
	}
}
