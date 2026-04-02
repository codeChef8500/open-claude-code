package skill

import (
	"fmt"
	"sync"
)

// ManagedRegistry is a thread-safe skill registry with map-based lookup,
// validation, and search. It complements the simpler slice-based Registry
// in skilltool.go with richer lifecycle management.
type ManagedRegistry struct {
	mu     sync.RWMutex
	skills map[string]*Skill
}

// NewManagedRegistry creates an empty managed skill registry.
func NewManagedRegistry() *ManagedRegistry {
	return &ManagedRegistry{skills: make(map[string]*Skill)}
}

// NewManagedRegistryFrom creates a registry pre-populated with the given skills.
func NewManagedRegistryFrom(skills []*Skill) *ManagedRegistry {
	r := NewManagedRegistry()
	for _, s := range skills {
		r.skills[s.Meta.Name] = s
	}
	return r
}

// Register adds a skill. Returns error on duplicate name.
func (r *ManagedRegistry) Register(s *Skill) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.skills[s.Meta.Name]; exists {
		return fmt.Errorf("skill %q already registered", s.Meta.Name)
	}
	r.skills[s.Meta.Name] = s
	return nil
}

// Upsert adds or replaces a skill by name.
func (r *ManagedRegistry) Upsert(s *Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[s.Meta.Name] = s
}

// Unregister removes a skill by name.
func (r *ManagedRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.skills, name)
}

// Get returns a skill by name.
func (r *ManagedRegistry) Get(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	return s, ok
}

// All returns all registered skills.
func (r *ManagedRegistry) All() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	return out
}

// Names returns all registered skill names.
func (r *ManagedRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}
	return names
}

// Count returns the number of registered skills.
func (r *ManagedRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.skills)
}

// Search searches registered skills by query.
func (r *ManagedRegistry) Search(query string) []SearchResult {
	return Search(r.All(), query)
}

// Validate checks a skill for common issues.
func Validate(s *Skill) []string {
	var issues []string
	if s.Meta.Name == "" {
		issues = append(issues, "name is required")
	}
	if s.Meta.Description == "" {
		issues = append(issues, "description is recommended")
	}
	if s.Prompt == "" && s.RawMD == "" {
		issues = append(issues, "skill has no content")
	}
	if len(s.Meta.Name) > 64 {
		issues = append(issues, "name exceeds 64 characters")
	}
	return issues
}
