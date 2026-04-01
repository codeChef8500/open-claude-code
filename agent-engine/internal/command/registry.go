package command

import (
	"fmt"
	"sort"
	"strings"
)

// Registry holds all registered slash commands.
type Registry struct {
	commands map[string]Command
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{commands: make(map[string]Command)}
}

// Register adds one or more commands. Panics on duplicate name.
func (r *Registry) Register(cmds ...Command) {
	for _, c := range cmds {
		name := strings.ToLower(c.Name())
		if _, exists := r.commands[name]; exists {
			panic(fmt.Sprintf("command %q already registered", name))
		}
		r.commands[name] = c
	}
}

// Find looks up a command by name (case-insensitive). Returns nil if not found.
func (r *Registry) Find(name string) Command {
	return r.commands[strings.ToLower(name)]
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
