package agent

// Built-in agent definitions aligned with claude-code-main's builtInAgents.ts
// and the individual agent files in built-in/*.ts.

// GeneralPurposeAgent is the default full-access coding agent.
var GeneralPurposeAgent = AgentDefinition{
	AgentType: "general-purpose",
	Source:    SourceBuiltIn,
	WhenToUse: "General-purpose coding agent for any task. Has full access to all tools. " +
		"Use this for implementation, debugging, refactoring, or any task that requires writing code.",
	MaxTurns: 200,
	// AllowedTools nil = all tools available
}

// ExploreAgent is a read-only exploration and research agent.
var ExploreAgent = AgentDefinition{
	AgentType:     "explore",
	Source:        SourceBuiltIn,
	WhenToUse:     "Read-only exploration, research, and code analysis. Cannot modify files.",
	OmitClaudeMd:  true,
	OmitGitStatus: true,
	AllowedTools: []string{
		"Read", "Grep", "Glob", "Bash", "lsp",
		"WebSearch", "WebFetch", "ToolSearch",
	},
}

// PlanAgent is a planning and architecture-focused agent.
var PlanAgent = AgentDefinition{
	AgentType:     "plan",
	Source:        SourceBuiltIn,
	WhenToUse:     "Planning, architecture discussion, and design review. Read-only access to codebase.",
	OmitClaudeMd:  true,
	OmitGitStatus: true,
	AllowedTools: []string{
		"Read", "Grep", "Glob", "Bash",
		"WebSearch", "WebFetch",
	},
}

// VerificationAgent is a testing and verification agent.
var VerificationAgent = AgentDefinition{
	AgentType: "verify",
	Source:    SourceBuiltIn,
	WhenToUse: "Testing, verification, and validation. Can run tests and check results " +
		"but primarily focused on read operations and running existing test suites.",
	AllowedTools: []string{
		"Read", "Grep", "Glob", "Bash", "PowerShell",
		"WebSearch", "WebFetch",
	},
}

// ForkAgent is a synthetic agent definition for fork subagents.
// It is not listed in the agent catalog — it is used internally when
// the AgentTool decides to fork the current conversation context.
// PermissionMode "bubble" means permission prompts bubble up to parent.
var ForkAgent = AgentDefinition{
	AgentType:      "__fork__",
	Source:         SourceBuiltIn,
	WhenToUse:      "Fork current conversation context for parallel work in a worktree.",
	Background:     true,
	Isolation:      IsolationWorktree,
	PermissionMode: "bubble",
}

// ClaudeCodeGuideAgent provides guidance about Claude Code usage.
var ClaudeCodeGuideAgent = AgentDefinition{
	AgentType:    "claude-code-guide",
	Source:       SourceBuiltIn,
	WhenToUse:    "Answer questions about how to use Claude Code, its features, and best practices.",
	OmitClaudeMd: true,
	AllowedTools: []string{
		"Read", "Grep", "Glob",
	},
}

// GetBuiltInAgents returns the list of built-in agents.
// If coordinatorMode is true, coordinator-specific agents are included.
func GetBuiltInAgents(coordinatorMode bool) []AgentDefinition {
	agents := []AgentDefinition{
		GeneralPurposeAgent,
		ExploreAgent,
		PlanAgent,
		VerificationAgent,
		ClaudeCodeGuideAgent,
	}

	if coordinatorMode {
		agents = append(agents, getCoordinatorAgents()...)
	}

	return agents
}

// getCoordinatorAgents returns agents specific to coordinator mode.
func getCoordinatorAgents() []AgentDefinition {
	return []AgentDefinition{
		{
			AgentType:  "coordinator-worker",
			Source:     SourceBuiltIn,
			WhenToUse:  "Worker agent spawned by the coordinator to implement a specific task.",
			Background: true,
			Isolation:  IsolationWorktree,
		},
	}
}

// IsBuiltInAgent checks if the given agent type is a built-in agent.
func IsBuiltInAgent(agentType string) bool {
	for _, a := range GetBuiltInAgents(true) {
		if a.AgentType == agentType {
			return true
		}
	}
	return agentType == ForkAgent.AgentType
}

// IsForkAgent checks if the given agent type is the synthetic fork agent.
func IsForkAgent(agentType string) bool {
	return agentType == ForkAgent.AgentType
}
