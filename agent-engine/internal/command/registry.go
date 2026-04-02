package command

import (
	"fmt"
	"sort"
	"strings"
)

// Registry holds all registered slash commands.
type Registry struct {
	commands map[string]Command
	aliases  map[string]string // alias -> canonical name
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]Command),
		aliases:  make(map[string]string),
	}
}

// Register adds one or more commands. Panics on duplicate name.
// Also registers any aliases the command declares.
func (r *Registry) Register(cmds ...Command) {
	for _, c := range cmds {
		name := strings.ToLower(c.Name())
		if _, exists := r.commands[name]; exists {
			panic(fmt.Sprintf("command %q already registered", name))
		}
		r.commands[name] = c
		for _, alias := range c.Aliases() {
			r.aliases[strings.ToLower(alias)] = name
		}
	}
}

// RegisterAlias manually maps an alias to a command name.
func (r *Registry) RegisterAlias(alias, cmdName string) {
	r.aliases[strings.ToLower(alias)] = strings.ToLower(cmdName)
}

// Find looks up a command by name or alias (case-insensitive). Returns nil if not found.
func (r *Registry) Find(name string) Command {
	key := strings.ToLower(name)
	if cmd, ok := r.commands[key]; ok {
		return cmd
	}
	// Check aliases.
	if canonical, ok := r.aliases[key]; ok {
		return r.commands[canonical]
	}
	return nil
}

// IsSlashCommand reports whether the input starts with a known command or alias.
func (r *Registry) IsSlashCommand(input string) bool {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return false
	}
	parts := strings.Fields(input[1:])
	if len(parts) == 0 {
		return false
	}
	return r.Find(parts[0]) != nil
}

// All returns all commands sorted by name.
func (r *Registry) All() []Command {
	cmds := make([]Command, 0, len(r.commands))
	for _, c := range r.commands {
		cmds = append(cmds, c)
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].Name() < cmds[j].Name()
	})
	return cmds
}

// Enabled returns all commands that pass IsEnabled for the given context.
func (r *Registry) Enabled(ectx *ExecContext) []Command {
	var enabled []Command
	for _, c := range r.All() {
		if c.IsEnabled(ectx) {
			enabled = append(enabled, c)
		}
	}
	return enabled
}
