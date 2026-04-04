package command

// MeetsAvailabilityRequirement checks whether a command should be available
// in the given context. Aligned with claude-code-main's
// meetsAvailabilityRequirement() in commands.ts.
//
// If the command has no availability restrictions, it is always available.
// Otherwise at least one of the command's availability values must match
// the current environment.
func MeetsAvailabilityRequirement(cmd Command, env CommandAvailability) bool {
	avail := cmd.Availability()
	if len(avail) == 0 {
		return true // no restriction → always available
	}
	for _, a := range avail {
		if a == env {
			return true
		}
	}
	return false
}

// FilterVisible returns commands that are not hidden and meet availability.
func FilterVisible(cmds []Command, env CommandAvailability) []Command {
	var visible []Command
	for _, cmd := range cmds {
		if cmd.IsHidden() {
			continue
		}
		if !MeetsAvailabilityRequirement(cmd, env) {
			continue
		}
		visible = append(visible, cmd)
	}
	return visible
}

// FormatDescriptionWithSource formats a command description including its
// source annotation. Aligned with formatDescriptionWithSource() in commands.ts.
func FormatDescriptionWithSource(cmd Command) string {
	desc := cmd.Description()
	src := cmd.Source()
	switch src {
	case CommandSourceBuiltin:
		return desc
	case CommandSourceBundled:
		return desc + " (bundled)"
	case CommandSourcePlugin:
		return desc + " (plugin)"
	case CommandSourceMCP:
		return desc + " (MCP)"
	case CommandSourceUser, CommandSourceCustom:
		return desc + " (custom)"
	case CommandSourceSDK:
		return desc + " (SDK)"
	default:
		return desc
	}
}
