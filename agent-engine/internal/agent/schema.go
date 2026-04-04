package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Agent definition validation aligned with claude-code-main's AgentJsonSchema
// in loadAgentsDir.ts. Validates agent definitions loaded from JSON/frontmatter.

// validIsolationModes is the set of allowed isolation values.
var validIsolationModes = map[string]bool{
	"":         true,
	"worktree": true,
	"remote":   true,
}

// validPermissionModes is the set of allowed permission mode values.
var validPermissionModes = map[string]bool{
	"":            true,
	"default":     true,
	"auto":        true,
	"bypass":      true,
	"acceptEdits": true,
	"plan":        true,
	"dontAsk":     true,
	"bubble":      true,
}

// validMemoryScopes is the set of allowed memory scope values.
var validMemoryScopes = map[string]bool{
	"":        true,
	"user":    true,
	"project": true,
	"local":   true,
}

// validEffortLevels is the set of allowed effort values.
var validEffortLevels = map[string]bool{
	"":       true,
	"low":    true,
	"medium": true,
	"high":   true,
}

// ValidateAgentDefinition validates an AgentDefinition against the schema rules.
// Returns nil if valid, or an error describing what is wrong.
func ValidateAgentDefinition(def *AgentDefinition) error {
	if def == nil {
		return fmt.Errorf("agent definition is nil")
	}

	var errs []string

	// agent_type is required and must not be empty.
	if def.AgentType == "" {
		errs = append(errs, "agent_type is required")
	}

	// Validate isolation mode.
	if !validIsolationModes[string(def.Isolation)] {
		errs = append(errs, fmt.Sprintf("invalid isolation mode %q", def.Isolation))
	}

	// Validate permission mode.
	if !validPermissionModes[def.PermissionMode] {
		errs = append(errs, fmt.Sprintf("invalid permission_mode %q", def.PermissionMode))
	}

	// Validate memory scope.
	if !validMemoryScopes[def.Memory] {
		errs = append(errs, fmt.Sprintf("invalid memory scope %q", def.Memory))
	}

	// Validate effort.
	if !validEffortLevels[def.Effort] {
		errs = append(errs, fmt.Sprintf("invalid effort level %q", def.Effort))
	}

	// max_turns must be non-negative.
	if def.MaxTurns < 0 {
		errs = append(errs, "max_turns must be non-negative")
	}

	// AllowedTools and DisallowedTools should not overlap.
	if len(def.AllowedTools) > 0 && len(def.DisallowedTools) > 0 {
		allowSet := make(map[string]bool)
		for _, t := range def.AllowedTools {
			allowSet[strings.ToLower(t)] = true
		}
		for _, t := range def.DisallowedTools {
			if allowSet[strings.ToLower(t)] {
				errs = append(errs, fmt.Sprintf("tool %q appears in both allowed_tools and disallowed_tools", t))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid agent definition %q: %s", def.AgentType, strings.Join(errs, "; "))
	}
	return nil
}

// ValidateAgentJSON validates agent definitions from a JSON byte slice.
// Expects either a single object or a map of agentType → definition.
func ValidateAgentJSON(data []byte) ([]AgentDefinition, error) {
	// Try as map first (multi-agent JSON config).
	var agentMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &agentMap); err == nil && len(agentMap) > 0 {
		var result []AgentDefinition
		for agentType, raw := range agentMap {
			var def AgentDefinition
			if err := json.Unmarshal(raw, &def); err != nil {
				return nil, fmt.Errorf("parse agent %q: %w", agentType, err)
			}
			if def.AgentType == "" {
				def.AgentType = agentType
			}
			if err := ValidateAgentDefinition(&def); err != nil {
				return nil, err
			}
			result = append(result, def)
		}
		return result, nil
	}

	// Try as single object.
	var def AgentDefinition
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse agent definition: %w", err)
	}
	if err := ValidateAgentDefinition(&def); err != nil {
		return nil, err
	}
	return []AgentDefinition{def}, nil
}

// ParseAgentToolSpec parses an Agent(x,y) tool specification string and
// returns the allowed agent types. For example, "Agent(explore,plan)" returns
// ["explore", "plan"]. Returns nil for plain "Agent" or "Task".
func ParseAgentToolSpec(spec string) []string {
	spec = strings.TrimSpace(spec)

	// Check for Agent(x,y) or Task(x,y) pattern.
	for _, prefix := range []string{"Agent(", "Task("} {
		if strings.HasPrefix(spec, prefix) && strings.HasSuffix(spec, ")") {
			inner := spec[len(prefix) : len(spec)-1]
			parts := strings.Split(inner, ",")
			var types []string
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					types = append(types, p)
				}
			}
			return types
		}
	}

	return nil
}
