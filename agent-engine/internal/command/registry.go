package command

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// DynamicLoader is a function that returns commands from an external source
// (skill directories, plugins, MCP servers, bundled skills, etc.).
// Aligned with claude-code-main's loadAllCommands() dynamic sources.
type DynamicLoader func() []Command

// Registry holds all registered slash commands.
// Thread-safe for concurrent reads and writes.
type Registry struct {
	mu       sync.RWMutex
	commands map[string]Command
	aliases  map[string]string // alias -> canonical name

	// Dynamic loaders are invoked by LoadDynamic to merge external commands.
	loaders []DynamicLoader

	// Cached sorted list, invalidated on mutation.
	cachedAll     []Command
	cachedVersion int
	version       int
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
	r.mu.Lock()
	defer r.mu.Unlock()
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
	r.version++
}

// RegisterOrReplace adds a command, replacing any existing command with the
// same name. Used by dynamic loaders to update commands at runtime.
func (r *Registry) RegisterOrReplace(cmds ...Command) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range cmds {
		name := strings.ToLower(c.Name())
		r.commands[name] = c
		for _, alias := range c.Aliases() {
			r.aliases[strings.ToLower(alias)] = name
		}
	}
	r.version++
}

// Unregister removes a command by name if it exists.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := strings.ToLower(name)
	cmd, ok := r.commands[key]
	if !ok {
		return
	}
	delete(r.commands, key)
	// Remove aliases pointing to this command.
	for _, alias := range cmd.Aliases() {
		delete(r.aliases, strings.ToLower(alias))
	}
	r.version++
}

// RegisterAlias manually maps an alias to a command name.
func (r *Registry) RegisterAlias(alias, cmdName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.aliases[strings.ToLower(alias)] = strings.ToLower(cmdName)
	r.version++
}

// AddLoader registers a DynamicLoader to be invoked by LoadDynamic.
func (r *Registry) AddLoader(loader DynamicLoader) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loaders = append(r.loaders, loader)
}

// LoadDynamic invokes all registered DynamicLoaders and merges their
// commands into the registry using RegisterOrReplace semantics.
// Aligned with claude-code-main loadAllCommands() merging behavior.
func (r *Registry) LoadDynamic() {
	r.mu.RLock()
	loaders := make([]DynamicLoader, len(r.loaders))
	copy(loaders, r.loaders)
	r.mu.RUnlock()

	for _, loader := range loaders {
		cmds := loader()
		if len(cmds) > 0 {
			r.RegisterOrReplace(cmds...)
		}
	}
}

// Find looks up a command by name or alias (case-insensitive). Returns nil if not found.
func (r *Registry) Find(name string) Command {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.findLocked(name)
}

func (r *Registry) findLocked(name string) Command {
	key := strings.ToLower(name)
	if cmd, ok := r.commands[key]; ok {
		return cmd
	}
	if canonical, ok := r.aliases[key]; ok {
		return r.commands[canonical]
	}
	return nil
}

// FindByPrefix returns all commands whose name starts with the given prefix.
// Used for autocomplete/tab-completion.
func (r *Registry) FindByPrefix(prefix string) []Command {
	r.mu.RLock()
	defer r.mu.RUnlock()
	prefix = strings.ToLower(prefix)
	var matches []Command
	for name, cmd := range r.commands {
		if strings.HasPrefix(name, prefix) {
			matches = append(matches, cmd)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Name() < matches[j].Name()
	})
	return matches
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

// All returns all commands sorted by name. Results are cached until mutation.
func (r *Registry) All() []Command {
	r.mu.RLock()
	if r.cachedVersion == r.version && r.cachedAll != nil {
		cached := r.cachedAll
		r.mu.RUnlock()
		return cached
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	// Double-check after acquiring write lock.
	if r.cachedVersion == r.version && r.cachedAll != nil {
		return r.cachedAll
	}
	cmds := make([]Command, 0, len(r.commands))
	for _, c := range r.commands {
		cmds = append(cmds, c)
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].Name() < cmds[j].Name()
	})
	r.cachedAll = cmds
	r.cachedVersion = r.version
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

// VisibleFor returns enabled, non-hidden commands that meet the given
// availability requirement. Aligned with claude-code-main getCommands().
func (r *Registry) VisibleFor(ectx *ExecContext, env CommandAvailability) []Command {
	var visible []Command
	for _, c := range r.All() {
		if !c.IsEnabled(ectx) {
			continue
		}
		if c.IsHidden() {
			continue
		}
		if !MeetsAvailabilityRequirement(c, env) {
			continue
		}
		visible = append(visible, c)
	}
	return visible
}

// Count returns the number of registered commands.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.commands)
}
