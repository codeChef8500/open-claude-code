package command

// CompletionEntry is a simplified representation of a command for TUI autocomplete.
// It avoids importing the tui package (which would create a cycle).
type CompletionEntry struct {
	Name        string
	Description string
	ArgHint     string
	Hidden      bool
	Aliases     []string
	Source      CommandSource
}

// CompletionEntries returns all visible commands as CompletionEntry values,
// suitable for bridging to the TUI's Completer. Hidden commands are excluded.
// If ectx is non-nil, availability filtering is applied.
func CompletionEntries(r *Registry, ectx *ExecContext) []CompletionEntry {
	if r == nil {
		r = Default()
	}

	var entries []CompletionEntry
	for _, cmd := range r.All() {
		if cmd.IsHidden() {
			continue
		}
		if ectx != nil && !cmd.IsEnabled(ectx) {
			continue
		}
		entry := CompletionEntry{
			Name:        cmd.Name(),
			Description: cmd.Description(),
			ArgHint:     cmd.ArgumentHint(),
			Hidden:      cmd.IsHidden(),
			Aliases:     cmd.Aliases(),
			Source:      cmd.Source(),
		}
		entries = append(entries, entry)

		// Also register aliases as separate entries so they appear in autocomplete.
		for _, alias := range cmd.Aliases() {
			entries = append(entries, CompletionEntry{
				Name:        alias,
				Description: cmd.Description(),
				ArgHint:     cmd.ArgumentHint(),
				Source:      cmd.Source(),
			})
		}
	}
	return entries
}
