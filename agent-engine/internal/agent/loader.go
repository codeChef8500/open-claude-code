package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/adrg/frontmatter"
)

// AgentLoader loads and merges agent definitions from multiple sources:
// built-in, custom (.claude/agents/*.md), and plugin (MCP-provided).
// Aligned with claude-code-main's loadAgentsDir.ts.
//
// Priority order (later overrides earlier by AgentType):
//
//	builtin → plugin → userSettings → projectSettings → custom → flagSettings → policySettings
type AgentLoader struct {
	mu                    sync.RWMutex
	builtInAgents         []AgentDefinition
	pluginAgents          []AgentDefinition
	userSettingsAgents    []AgentDefinition
	projectSettingsAgents []AgentDefinition
	customAgents          []AgentDefinition
	flagSettingsAgents    []AgentDefinition
	policySettingsAgents  []AgentDefinition
	merged                []AgentDefinition
	mergedDirty           bool
	availableMcp          map[string]bool // currently connected MCP server names
	failedFiles           []FailedAgentFile
}

// NewAgentLoader creates an AgentLoader pre-populated with built-in agents.
func NewAgentLoader() *AgentLoader {
	l := &AgentLoader{
		availableMcp: make(map[string]bool),
		mergedDirty:  true,
	}
	l.builtInAgents = GetBuiltInAgents(false)
	return l
}

// SetBuiltInAgents replaces the built-in agent list (e.g. when coordinator mode changes).
func (l *AgentLoader) SetBuiltInAgents(agents []AgentDefinition) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.builtInAgents = agents
	l.mergedDirty = true
}

// SetAvailableMcp updates the set of currently connected MCP servers.
func (l *AgentLoader) SetAvailableMcp(names []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.availableMcp = make(map[string]bool, len(names))
	for _, n := range names {
		l.availableMcp[n] = true
	}
	l.mergedDirty = true
}

// agentFrontmatter is the YAML frontmatter schema for custom agent .md files.
// Aligned with claude-code-main's AgentJsonSchema in loadAgentsDir.ts.
type agentFrontmatter struct {
	AgentType              string   `yaml:"agent_type"`
	WhenToUse              string   `yaml:"when_to_use"`
	Model                  string   `yaml:"model"`
	Effort                 string   `yaml:"effort"`
	MaxTurns               int      `yaml:"max_turns"`
	Background             bool     `yaml:"background"`
	Isolation              string   `yaml:"isolation"`
	Tools                  []string `yaml:"tools"`
	DisallowedTools        []string `yaml:"disallowed_tools"`
	Skills                 []string `yaml:"skills"`
	OmitClaudeMd           bool     `yaml:"omit_claude_md"`
	CriticalSystemReminder string   `yaml:"critical_system_reminder"`
	PermissionMode         string   `yaml:"permission_mode"`
	Memory                 string   `yaml:"memory"`
	RequiredMcpServers     []string `yaml:"required_mcp_servers"`
}

// LoadCustom scans projectDir/.claude/agents/*.md for custom agent definitions.
func (l *AgentLoader) LoadCustom(projectDir string) ([]AgentDefinition, error) {
	agentsDir := filepath.Join(projectDir, ".claude", "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read agents dir: %w", err)
	}

	var agents []AgentDefinition
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		def, err := l.parseAgentFile(filepath.Join(agentsDir, entry.Name()))
		if err != nil {
			continue // skip invalid files
		}
		agents = append(agents, def)
	}

	l.mu.Lock()
	l.customAgents = agents
	l.mergedDirty = true
	l.mu.Unlock()

	return agents, nil
}

// parseAgentFile reads a single .md agent file with YAML frontmatter.
func (l *AgentLoader) parseAgentFile(path string) (AgentDefinition, error) {
	f, err := os.Open(path)
	if err != nil {
		return AgentDefinition{}, fmt.Errorf("open agent file: %w", err)
	}
	defer f.Close()

	var fm agentFrontmatter
	body, err := frontmatter.Parse(f, &fm)
	if err != nil {
		return AgentDefinition{}, fmt.Errorf("parse frontmatter: %w", err)
	}

	agentType := fm.AgentType
	if agentType == "" {
		// Default agent type from filename (strip .md extension).
		agentType = strings.TrimSuffix(filepath.Base(path), ".md")
	}

	def := AgentDefinition{
		AgentType:              agentType,
		Source:                 SourceCustom,
		WhenToUse:              fm.WhenToUse,
		SystemPrompt:           strings.TrimSpace(string(body)),
		Model:                  fm.Model,
		Effort:                 fm.Effort,
		MaxTurns:               fm.MaxTurns,
		Background:             fm.Background,
		Isolation:              IsolationMode(fm.Isolation),
		AllowedTools:           fm.Tools,
		DisallowedTools:        fm.DisallowedTools,
		Skills:                 fm.Skills,
		OmitClaudeMd:           fm.OmitClaudeMd,
		CriticalSystemReminder: fm.CriticalSystemReminder,
		PermissionMode:         fm.PermissionMode,
		Memory:                 fm.Memory,
		RequiredMcpServers:     fm.RequiredMcpServers,
	}

	return def, nil
}

// AddPluginAgents adds agents discovered from MCP tool servers.
func (l *AgentLoader) AddPluginAgents(agents []AgentDefinition) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range agents {
		agents[i].Source = SourcePlugin
	}
	l.pluginAgents = agents
	l.mergedDirty = true
}

// SetUserSettingsAgents sets agents from user-level settings.
func (l *AgentLoader) SetUserSettingsAgents(agents []AgentDefinition) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range agents {
		agents[i].Source = SourceUserSettings
	}
	l.userSettingsAgents = agents
	l.mergedDirty = true
}

// SetProjectSettingsAgents sets agents from project-level settings.
func (l *AgentLoader) SetProjectSettingsAgents(agents []AgentDefinition) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range agents {
		agents[i].Source = SourceProjectSettings
	}
	l.projectSettingsAgents = agents
	l.mergedDirty = true
}

// SetFlagSettingsAgents sets agents from feature flag settings.
func (l *AgentLoader) SetFlagSettingsAgents(agents []AgentDefinition) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range agents {
		agents[i].Source = SourceFlagSettings
	}
	l.flagSettingsAgents = agents
	l.mergedDirty = true
}

// SetPolicySettingsAgents sets agents from policy settings (highest priority).
func (l *AgentLoader) SetPolicySettingsAgents(agents []AgentDefinition) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range agents {
		agents[i].Source = SourcePolicySettings
	}
	l.policySettingsAgents = agents
	l.mergedDirty = true
}

// LoadFromJSON loads agent definitions from a JSON byte slice.
// Used for settings-based agent configuration.
func (l *AgentLoader) LoadFromJSON(data []byte, source AgentDefinitionSource) ([]AgentDefinition, error) {
	agents, err := ValidateAgentJSON(data)
	if err != nil {
		return nil, err
	}
	for i := range agents {
		agents[i].Source = source
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	switch source {
	case SourceUserSettings:
		l.userSettingsAgents = agents
	case SourceProjectSettings:
		l.projectSettingsAgents = agents
	case SourceFlagSettings:
		l.flagSettingsAgents = agents
	case SourcePolicySettings:
		l.policySettingsAgents = agents
	case SourcePlugin:
		l.pluginAgents = agents
	default:
		l.customAgents = agents
	}
	l.mergedDirty = true

	return agents, nil
}

// FailedFiles returns the list of agent files that failed to load.
func (l *AgentLoader) FailedFiles() []FailedAgentFile {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]FailedAgentFile, len(l.failedFiles))
	copy(result, l.failedFiles)
	return result
}

// MergeAll returns the merged list of all agent definitions.
// Priority order (later overrides earlier by AgentType):
//
//	builtin → plugin → userSettings → projectSettings → custom → flagSettings → policySettings
func (l *AgentLoader) MergeAll() []AgentDefinition {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.mergedDirty {
		return l.merged
	}

	seen := make(map[string]int) // agentType → index in result
	var result []AgentDefinition

	// Helper to add/override agents.
	mergeLayer := func(agents []AgentDefinition) {
		for _, a := range agents {
			if idx, ok := seen[a.AgentType]; ok {
				result[idx] = a
			} else {
				seen[a.AgentType] = len(result)
				result = append(result, a)
			}
		}
	}

	// Apply in priority order (lowest → highest).
	mergeLayer(l.builtInAgents)
	mergeLayer(l.pluginAgents)
	mergeLayer(l.userSettingsAgents)
	mergeLayer(l.projectSettingsAgents)
	mergeLayer(l.customAgents)
	mergeLayer(l.flagSettingsAgents)
	mergeLayer(l.policySettingsAgents)

	l.merged = result
	l.mergedDirty = false
	return l.merged
}

// MergeAllResult returns the full merge result including failed files.
func (l *AgentLoader) MergeAllResult() *AgentDefinitionsResult {
	all := l.MergeAll()
	active := l.FilterByMcpAvailability(all)

	return &AgentDefinitionsResult{
		ActiveAgents: active,
		AllAgents:    all,
		FailedFiles:  l.FailedFiles(),
	}
}

// FindByType returns the agent definition matching the given type.
func (l *AgentLoader) FindByType(agentType string) (*AgentDefinition, bool) {
	all := l.MergeAll()
	for i := range all {
		if strings.EqualFold(all[i].AgentType, agentType) {
			return &all[i], true
		}
	}
	return nil, false
}

// FilterByMcpAvailability returns only agents whose required MCP servers are available.
func (l *AgentLoader) FilterByMcpAvailability(agents []AgentDefinition) []AgentDefinition {
	l.mu.RLock()
	mcp := l.availableMcp
	l.mu.RUnlock()

	var result []AgentDefinition
	for _, a := range agents {
		if l.mcpRequirementsMet(a, mcp) {
			result = append(result, a)
		}
	}
	return result
}

// mcpRequirementsMet checks if all required MCP servers for an agent are available.
func (l *AgentLoader) mcpRequirementsMet(def AgentDefinition, available map[string]bool) bool {
	for _, pattern := range def.RequiredMcpServers {
		found := false
		for name := range available {
			if matchMcpPattern(pattern, name) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// matchMcpPattern matches an MCP server name against a pattern.
// Supports simple glob: "*" matches everything, "prefix*" matches prefix.
func matchMcpPattern(pattern, name string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(name, strings.TrimSuffix(pattern, "*"))
	}
	return strings.EqualFold(pattern, name)
}

// AvailableAgentTypes returns the list of agent type names currently available.
func (l *AgentLoader) AvailableAgentTypes() []string {
	all := l.MergeAll()
	filtered := l.FilterByMcpAvailability(all)
	types := make([]string, len(filtered))
	for i, a := range filtered {
		types[i] = a.AgentType
	}
	return types
}
