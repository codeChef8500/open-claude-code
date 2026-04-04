package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Team management aligned with claude-code-main's team.ts / TeamFile.
//
// A team is a named group of agents that collaborate on a shared task.
// The team file persists team state to disk for recovery and inspection.

// TeamFile represents the on-disk team configuration and state.
type TeamFile struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	LeadAgent   string       `json:"lead_agent"`
	Members     []TeamMember `json:"members"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Status      TeamStatus   `json:"status"`
}

// TeamMember is a single agent participating in a team.
type TeamMember struct {
	AgentID   string `json:"agent_id"`
	AgentType string `json:"agent_type"`
	Role      string `json:"role,omitempty"` // "lead", "worker", "observer"
	Status    string `json:"status"`         // "active", "idle", "stopped"
}

// TeamStatus represents the lifecycle of a team.
type TeamStatus string

const (
	TeamStatusActive   TeamStatus = "active"
	TeamStatusPaused   TeamStatus = "paused"
	TeamStatusFinished TeamStatus = "finished"
	TeamStatusFailed   TeamStatus = "failed"
)

// TeamManager manages team creation, membership, and persistence.
type TeamManager struct {
	mu       sync.RWMutex
	teams    map[string]*TeamFile
	baseDir  string // directory to store team files
	registry *MailboxRegistry
	bus      *MessageBus
}

// NewTeamManager creates a new team manager.
func NewTeamManager(baseDir string, registry *MailboxRegistry, bus *MessageBus) *TeamManager {
	return &TeamManager{
		teams:    make(map[string]*TeamFile),
		baseDir:  baseDir,
		registry: registry,
		bus:      bus,
	}
}

// CreateTeam creates a new team with the given name and lead agent.
func (tm *TeamManager) CreateTeam(name, description, leadAgentID string) (*TeamFile, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.teams[name]; exists {
		return nil, fmt.Errorf("team %q already exists", name)
	}

	team := &TeamFile{
		Name:        name,
		Description: description,
		LeadAgent:   leadAgentID,
		Members: []TeamMember{
			{
				AgentID: leadAgentID,
				Role:    "lead",
				Status:  "active",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Status:    TeamStatusActive,
	}

	tm.teams[name] = team

	// Persist to disk.
	if err := tm.saveTeam(team); err != nil {
		return nil, fmt.Errorf("save team: %w", err)
	}

	// Subscribe lead agent to message bus.
	if tm.bus != nil {
		_, _ = tm.bus.Subscribe(leadAgentID, 64)
	}

	return team, nil
}

// AddMember adds an agent to an existing team.
func (tm *TeamManager) AddMember(teamName, agentID, agentType, role string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	team, ok := tm.teams[teamName]
	if !ok {
		return fmt.Errorf("team %q not found", teamName)
	}

	// Check if already a member.
	for _, m := range team.Members {
		if m.AgentID == agentID {
			return nil // already a member
		}
	}

	if role == "" {
		role = "worker"
	}

	team.Members = append(team.Members, TeamMember{
		AgentID:   agentID,
		AgentType: agentType,
		Role:      role,
		Status:    "active",
	})
	team.UpdatedAt = time.Now()

	// Subscribe to message bus.
	if tm.bus != nil {
		_, _ = tm.bus.Subscribe(agentID, 64)
	}

	// Create mailbox.
	if tm.registry != nil {
		tm.registry.GetOrCreate(agentID)
	}

	return tm.saveTeam(team)
}

// RemoveMember removes an agent from a team.
func (tm *TeamManager) RemoveMember(teamName, agentID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	team, ok := tm.teams[teamName]
	if !ok {
		return fmt.Errorf("team %q not found", teamName)
	}

	filtered := team.Members[:0]
	for _, m := range team.Members {
		if m.AgentID != agentID {
			filtered = append(filtered, m)
		}
	}
	team.Members = filtered
	team.UpdatedAt = time.Now()

	// Unsubscribe from message bus.
	if tm.bus != nil {
		tm.bus.Unsubscribe(agentID)
	}

	// Remove mailbox.
	if tm.registry != nil {
		tm.registry.Remove(agentID)
	}

	return tm.saveTeam(team)
}

// GetTeam returns a team by name.
func (tm *TeamManager) GetTeam(name string) (*TeamFile, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	t, ok := tm.teams[name]
	return t, ok
}

// ListTeams returns all teams.
func (tm *TeamManager) ListTeams() []*TeamFile {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	result := make([]*TeamFile, 0, len(tm.teams))
	for _, t := range tm.teams {
		result = append(result, t)
	}
	return result
}

// BroadcastToTeam sends a message to all members of a team.
func (tm *TeamManager) BroadcastToTeam(teamName, fromAgentID, message string) error {
	tm.mu.RLock()
	team, ok := tm.teams[teamName]
	tm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("team %q not found", teamName)
	}

	var lastErr error
	for _, m := range team.Members {
		if m.AgentID == fromAgentID {
			continue // don't send to self
		}
		if tm.registry != nil {
			_, err := tm.registry.Send(fromAgentID, m.AgentID, message, MailboxPriorityNormal, "")
			if err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

// FinishTeam marks a team as finished.
func (tm *TeamManager) FinishTeam(teamName string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	team, ok := tm.teams[teamName]
	if !ok {
		return fmt.Errorf("team %q not found", teamName)
	}

	team.Status = TeamStatusFinished
	team.UpdatedAt = time.Now()

	for i := range team.Members {
		team.Members[i].Status = "stopped"
	}

	return tm.saveTeam(team)
}

// LoadTeam loads a team from disk.
func (tm *TeamManager) LoadTeam(name string) (*TeamFile, error) {
	path := tm.teamFilePath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read team file: %w", err)
	}

	var team TeamFile
	if err := json.Unmarshal(data, &team); err != nil {
		return nil, fmt.Errorf("parse team file: %w", err)
	}

	tm.mu.Lock()
	tm.teams[name] = &team
	tm.mu.Unlock()

	return &team, nil
}

// saveTeam persists team state to disk.
func (tm *TeamManager) saveTeam(team *TeamFile) error {
	if tm.baseDir == "" {
		return nil // no persistence configured
	}

	dir := filepath.Join(tm.baseDir, ".claude", "teams")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create teams dir: %w", err)
	}

	data, err := json.MarshalIndent(team, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal team: %w", err)
	}

	return os.WriteFile(tm.teamFilePath(team.Name), data, 0o644)
}

// teamFilePath returns the file path for a team's state file.
func (tm *TeamManager) teamFilePath(name string) string {
	return filepath.Join(tm.baseDir, ".claude", "teams", name+".json")
}

// TeamMemberIDs returns the agent IDs for all members of a team.
func (tm *TeamManager) TeamMemberIDs(teamName string) []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	team, ok := tm.teams[teamName]
	if !ok {
		return nil
	}

	ids := make([]string, len(team.Members))
	for i, m := range team.Members {
		ids[i] = m.AgentID
	}
	return ids
}

// ── Extended Team Operations ──────────────────────────────────────────

// DeleteTeam removes a team, cleaning up all member subscriptions.
func (tm *TeamManager) DeleteTeam(teamName string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	team, ok := tm.teams[teamName]
	if !ok {
		return fmt.Errorf("team %q not found", teamName)
	}

	// Cleanup member subscriptions.
	for _, m := range team.Members {
		if tm.bus != nil {
			tm.bus.Unsubscribe(m.AgentID)
		}
		if tm.registry != nil {
			tm.registry.Remove(m.AgentID)
		}
	}

	delete(tm.teams, teamName)

	// Remove file from disk.
	path := tm.teamFilePath(teamName)
	_ = os.Remove(path)

	slog.Info("team: deleted", slog.String("team", teamName))
	return nil
}

// SendMessageToMember sends a direct message to a specific team member.
func (tm *TeamManager) SendMessageToMember(teamName, fromAgentID, toAgentID, message string) (string, error) {
	tm.mu.RLock()
	team, ok := tm.teams[teamName]
	tm.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("team %q not found", teamName)
	}

	// Verify recipient is a member.
	found := false
	for _, m := range team.Members {
		if m.AgentID == toAgentID {
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("agent %s is not a member of team %q", toAgentID, teamName)
	}

	if tm.registry == nil {
		return "", fmt.Errorf("mailbox registry not configured")
	}

	return tm.registry.Send(fromAgentID, toAgentID, message, MailboxPriorityNormal, "")
}

// UpdateMemberStatus updates the status of a specific team member.
func (tm *TeamManager) UpdateMemberStatus(teamName, agentID, status string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	team, ok := tm.teams[teamName]
	if !ok {
		return fmt.Errorf("team %q not found", teamName)
	}

	for i := range team.Members {
		if team.Members[i].AgentID == agentID {
			team.Members[i].Status = status
			team.UpdatedAt = time.Now()
			return tm.saveTeam(team)
		}
	}

	return fmt.Errorf("agent %s not found in team %q", agentID, teamName)
}

// LoadAllTeams loads all team files from disk into memory.
func (tm *TeamManager) LoadAllTeams() error {
	dir := filepath.Join(tm.baseDir, ".claude", "teams")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read teams dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		if _, err := tm.LoadTeam(name); err != nil {
			slog.Warn("team: failed to load",
				slog.String("team", name),
				slog.Any("err", err),
			)
		}
	}
	return nil
}

// FormatTeamStatus returns a human-readable status summary for a team.
func (tm *TeamManager) FormatTeamStatus(teamName string) string {
	tm.mu.RLock()
	team, ok := tm.teams[teamName]
	tm.mu.RUnlock()

	if !ok {
		return fmt.Sprintf("Team %q not found.", teamName)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Team: %s\n", team.Name))
	if team.Description != "" {
		sb.WriteString(fmt.Sprintf("%s\n", team.Description))
	}
	sb.WriteString(fmt.Sprintf("Status: %s | Lead: %s\n", team.Status, truncID(team.LeadAgent)))
	sb.WriteString(fmt.Sprintf("Members: %d\n\n", len(team.Members)))

	for _, m := range team.Members {
		role := m.Role
		if role == "" {
			role = "worker"
		}
		sb.WriteString(fmt.Sprintf("  - [%s] %s (%s) - %s\n",
			m.Status, truncID(m.AgentID), m.AgentType, role))
	}

	return sb.String()
}

// GetLeadAgent returns the lead agent ID for a team.
func (tm *TeamManager) GetLeadAgent(teamName string) (string, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	team, ok := tm.teams[teamName]
	if !ok {
		return "", fmt.Errorf("team %q not found", teamName)
	}
	return team.LeadAgent, nil
}

// ActiveMemberCount returns the number of active members in a team.
func (tm *TeamManager) ActiveMemberCount(teamName string) int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	team, ok := tm.teams[teamName]
	if !ok {
		return 0
	}

	count := 0
	for _, m := range team.Members {
		if m.Status == "active" {
			count++
		}
	}
	return count
}

// FindTeamByMember returns the team name that contains the given agent ID.
func (tm *TeamManager) FindTeamByMember(agentID string) (string, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	for name, team := range tm.teams {
		for _, m := range team.Members {
			if m.AgentID == agentID {
				return name, true
			}
		}
	}
	return "", false
}
