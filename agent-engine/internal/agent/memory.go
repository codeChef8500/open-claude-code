package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Agent memory system aligned with claude-code-main's agentMemory.ts.
//
// Memory scopes:
//   - user:    ~/.claude/agent-memory/<agent_type>.md  (global per user)
//   - project: <project>/.claude/agent-memory/<agent_type>.md  (per project)
//   - local:   <project>/.claude/local/agent-memory/<agent_type>.md  (gitignored)

// MemoryScope defines where agent memory is stored.
type MemoryScope string

const (
	MemoryScopeUser    MemoryScope = "user"
	MemoryScopeProject MemoryScope = "project"
	MemoryScopeLocal   MemoryScope = "local"
)

// AgentMemoryManager handles reading and writing agent-specific memory.
type AgentMemoryManager struct {
	mu         sync.RWMutex
	homeDir    string
	projectDir string
	cache      map[string]string // key: scope+agentType → content
}

// NewAgentMemoryManager creates a new memory manager.
func NewAgentMemoryManager(homeDir, projectDir string) *AgentMemoryManager {
	return &AgentMemoryManager{
		homeDir:    homeDir,
		projectDir: projectDir,
		cache:      make(map[string]string),
	}
}

// memoryPath returns the file path for a given scope and agent type.
func (m *AgentMemoryManager) memoryPath(scope MemoryScope, agentType string) string {
	filename := sanitizeAgentType(agentType) + ".md"

	switch scope {
	case MemoryScopeUser:
		return filepath.Join(m.homeDir, ".claude", "agent-memory", filename)
	case MemoryScopeProject:
		return filepath.Join(m.projectDir, ".claude", "agent-memory", filename)
	case MemoryScopeLocal:
		return filepath.Join(m.projectDir, ".claude", "local", "agent-memory", filename)
	default:
		return filepath.Join(m.projectDir, ".claude", "agent-memory", filename)
	}
}

// Load reads agent memory for the given scope and agent type.
// Returns empty string if no memory exists.
func (m *AgentMemoryManager) Load(scope MemoryScope, agentType string) (string, error) {
	m.mu.RLock()
	key := string(scope) + ":" + agentType
	if cached, ok := m.cache[key]; ok {
		m.mu.RUnlock()
		return cached, nil
	}
	m.mu.RUnlock()

	path := m.memoryPath(scope, agentType)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read agent memory: %w", err)
	}

	content := string(data)

	m.mu.Lock()
	m.cache[key] = content
	m.mu.Unlock()

	return content, nil
}

// Save writes agent memory for the given scope and agent type.
func (m *AgentMemoryManager) Save(scope MemoryScope, agentType, content string) error {
	path := m.memoryPath(scope, agentType)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write agent memory: %w", err)
	}

	m.mu.Lock()
	key := string(scope) + ":" + agentType
	m.cache[key] = content
	m.mu.Unlock()

	return nil
}

// Append adds content to existing memory (separated by newline).
func (m *AgentMemoryManager) Append(scope MemoryScope, agentType, content string) error {
	existing, err := m.Load(scope, agentType)
	if err != nil {
		return err
	}

	var newContent string
	if existing == "" {
		newContent = content
	} else {
		newContent = existing + "\n\n" + content
	}

	return m.Save(scope, agentType, newContent)
}

// Delete removes agent memory for the given scope and agent type.
func (m *AgentMemoryManager) Delete(scope MemoryScope, agentType string) error {
	path := m.memoryPath(scope, agentType)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete agent memory: %w", err)
	}

	m.mu.Lock()
	key := string(scope) + ":" + agentType
	delete(m.cache, key)
	m.mu.Unlock()

	return nil
}

// LoadAll loads memory from all applicable scopes for an agent type.
// Returns combined content with scope headers.
func (m *AgentMemoryManager) LoadAll(agentType string) string {
	var parts []string

	for _, scope := range []MemoryScope{MemoryScopeUser, MemoryScopeProject, MemoryScopeLocal} {
		content, err := m.Load(scope, agentType)
		if err != nil || content == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("## Agent Memory (%s)\n\n%s", scope, content))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n\n---\n\n")
}

// BuildMemoryPrompt constructs the memory section for an agent's system prompt.
// Aligned with claude-code-main's getAgentMemoryPrompt.
func (m *AgentMemoryManager) BuildMemoryPrompt(def *AgentDefinition) string {
	if def == nil || def.Memory == "" {
		return ""
	}

	agentType := def.AgentType
	scope := MemoryScope(def.Memory)

	// If scope is a specific scope, load just that one.
	if scope == MemoryScopeUser || scope == MemoryScopeProject || scope == MemoryScopeLocal {
		content, _ := m.Load(scope, agentType)
		if content == "" {
			return ""
		}
		return fmt.Sprintf("\n\n# Agent Memory\n\n%s", content)
	}

	// Otherwise, load all scopes.
	all := m.LoadAll(agentType)
	if all == "" {
		return ""
	}
	return fmt.Sprintf("\n\n# Agent Memory\n\n%s", all)
}

// InvalidateCache clears the memory cache.
func (m *AgentMemoryManager) InvalidateCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache = make(map[string]string)
}

// ListMemories returns all agent types that have memory in a given scope.
func (m *AgentMemoryManager) ListMemories(scope MemoryScope) ([]string, error) {
	var dir string
	switch scope {
	case MemoryScopeUser:
		dir = filepath.Join(m.homeDir, ".claude", "agent-memory")
	case MemoryScopeProject:
		dir = filepath.Join(m.projectDir, ".claude", "agent-memory")
	case MemoryScopeLocal:
		dir = filepath.Join(m.projectDir, ".claude", "local", "agent-memory")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var types []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		types = append(types, strings.TrimSuffix(e.Name(), ".md"))
	}
	return types, nil
}

// sanitizeAgentType makes an agent type safe for use as a filename.
func sanitizeAgentType(agentType string) string {
	// Replace path separators and special chars.
	r := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
		" ", "-",
	)
	return strings.ToLower(r.Replace(agentType))
}

// ── Memory Snapshot ──────────────────────────────────────────────────────
// Captures and restores agent memory state across sessions.

// MemoryEntry holds memory for a single agent type in a single scope.
type MemoryEntry struct {
	Scope     MemoryScope `json:"scope"`
	AgentType string      `json:"agent_type"`
	Content   string      `json:"content"`
}

// MemorySnapshot captures the complete memory state at a point in time.
type MemorySnapshot struct {
	SessionID string        `json:"session_id"`
	Entries   []MemoryEntry `json:"entries"`
	CreatedAt time.Time     `json:"created_at"`
}

// TakeSnapshot captures all current memory entries into a snapshot.
func (m *AgentMemoryManager) TakeSnapshot(sessionID string) (*MemorySnapshot, error) {
	snapshot := &MemorySnapshot{
		SessionID: sessionID,
		CreatedAt: time.Now(),
	}

	for _, scope := range []MemoryScope{MemoryScopeUser, MemoryScopeProject, MemoryScopeLocal} {
		types, err := m.ListMemories(scope)
		if err != nil {
			continue
		}
		for _, agentType := range types {
			content, err := m.Load(scope, agentType)
			if err != nil || content == "" {
				continue
			}
			snapshot.Entries = append(snapshot.Entries, MemoryEntry{
				Scope:     scope,
				AgentType: agentType,
				Content:   content,
			})
		}
	}

	return snapshot, nil
}

// RestoreSnapshot restores memory from a snapshot, overwriting current memory.
func (m *AgentMemoryManager) RestoreSnapshot(snapshot *MemorySnapshot) error {
	for _, entry := range snapshot.Entries {
		if err := m.Save(entry.Scope, entry.AgentType, entry.Content); err != nil {
			return fmt.Errorf("restore memory %s/%s: %w", entry.Scope, entry.AgentType, err)
		}
	}
	return nil
}

// SaveSnapshotToFile writes a snapshot to a JSON file.
func SaveSnapshotToFile(snapshot *MemorySnapshot, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// LoadSnapshotFromFile reads a snapshot from a JSON file.
func LoadSnapshotFromFile(path string) (*MemorySnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snapshot MemorySnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

// BuildMemoryPromptMultiAgent builds a combined memory prompt for multiple
// agent types. Used in team/swarm contexts where the coordinator needs
// memory from several agent types.
func (m *AgentMemoryManager) BuildMemoryPromptMultiAgent(agentTypes []string) string {
	var parts []string

	for _, agentType := range agentTypes {
		all := m.LoadAll(agentType)
		if all == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("### Memory for %s\n\n%s", agentType, all))
	}

	if len(parts) == 0 {
		return ""
	}

	return fmt.Sprintf("\n\n# Agent Memory\n\n%s", strings.Join(parts, "\n\n---\n\n"))
}
