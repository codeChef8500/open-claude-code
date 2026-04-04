package command

import "context"

// ─── /agents ─────────────────────────────────────────────────────────────────

// AgentsCommand manages agent configurations.
// Aligned with claude-code-main commands/agents/index.ts (local-jsx).
type AgentsCommand struct{ BaseCommand }

func (c *AgentsCommand) Name() string                  { return "agents" }
func (c *AgentsCommand) Description() string           { return "Manage agent configurations" }
func (c *AgentsCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *AgentsCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *AgentsCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "agents"}, nil
}

// ─── /tasks ──────────────────────────────────────────────────────────────────

// TasksCommand lists and manages background tasks.
// Aligned with claude-code-main commands/tasks/index.ts (local-jsx).
type TasksCommand struct{ BaseCommand }

func (c *TasksCommand) Name() string                  { return "tasks" }
func (c *TasksCommand) Aliases() []string             { return []string{"bashes"} }
func (c *TasksCommand) Description() string           { return "List and manage background tasks" }
func (c *TasksCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *TasksCommand) IsEnabled(_ *ExecContext) bool { return true }
func (c *TasksCommand) ExecuteInteractive(_ context.Context, _ []string, _ *ExecContext) (*InteractiveResult, error) {
	return &InteractiveResult{Component: "tasks"}, nil
}

func init() {
	defaultRegistry.Register(
		&AgentsCommand{},
		&TasksCommand{},
	)
}
