package plugin

import (
	"fmt"
	"sync"
)

// Registry manages loaded plugin instances.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]*ExternalPlugin
}

// NewRegistry creates an empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{plugins: make(map[string]*ExternalPlugin)}
}

// Register adds a plugin. Returns error on duplicate name.
func (r *Registry) Register(p *ExternalPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.plugins[p.meta.Name]; exists {
		return fmt.Errorf("plugin %q already registered", p.meta.Name)
	}
	r.plugins[p.meta.Name] = p
	return nil
}

// Unregister removes a plugin by name and kills its subprocess.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	p.Close()
	delete(r.plugins, name)
	return nil
}

// Get returns a plugin by name.
func (r *Registry) Get(name string) (*ExternalPlugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// All returns all registered plugins.
func (r *Registry) All() []*ExternalPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*ExternalPlugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, p)
	}
	return out
}

// CloseAll kills all plugin subprocesses and clears the registry.
func (r *Registry) CloseAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, p := range r.plugins {
		p.Close()
		delete(r.plugins, name)
	}
}
