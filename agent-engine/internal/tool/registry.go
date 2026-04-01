package tool

import (
	"sort"
	"sync"
)

// Registry holds all registered tools and assembles the active tool pool.
type Registry struct {
	mu    sync.RWMutex
	tools []Tool
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds one or more tools to the registry.
func (r *Registry) Register(tools ...Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools = append(r.tools, tools...)
}

// All returns all registered tools sorted by name (stable ordering for
// prompt cache stability).
func (r *Registry) All() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sorted := make([]Tool, len(r.tools))
	copy(sorted, r.tools)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Name() < sorted[j].Name()
	})
	return sorted
}

// Enabled returns only tools that pass IsEnabled for the given UseContext.
func (r *Registry) Enabled(uctx *UseContext) []Tool {
	all := r.All()
	var enabled []Tool
	for _, t := range all {
		if t.IsEnabled(uctx) {
			enabled = append(enabled, t)
		}
	}
	return enabled
}

// Find looks up a tool by name. Returns nil if not found.
func (r *Registry) Find(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, t := range r.tools {
		if t.Name() == name {
			return t
		}
	}
	return nil
}

// Len returns the number of registered tools.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}
