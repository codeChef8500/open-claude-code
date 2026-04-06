package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// Workflow command system deep implementation.
// Aligned with claude-code-main commands/workflow/* and skill-based workflows.
//
// Workflows are .md files in .claude/workflows/ that define reusable
// multi-step procedures with YAML frontmatter + markdown body.
// ──────────────────────────────────────────────────────────────────────────────

// WorkflowViewData is the structured data for the workflow TUI component.
type WorkflowViewData struct {
	Subcommand string             `json:"subcommand"` // "list", "run", "create", "edit", "delete"
	Workflows  []WorkflowEntry    `json:"workflows,omitempty"`
	Active     *WorkflowRunStatus `json:"active,omitempty"`
	Message    string             `json:"message,omitempty"`
	Error      string             `json:"error,omitempty"`
}

// WorkflowEntry describes a single workflow file.
type WorkflowEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	FilePath    string `json:"file_path"`
	StepCount   int    `json:"step_count"`
}

// WorkflowRunStatus tracks a running workflow.
type WorkflowRunStatus struct {
	Name        string `json:"name"`
	CurrentStep int    `json:"current_step"`
	TotalSteps  int    `json:"total_steps"`
	Status      string `json:"status"` // "running", "paused", "completed", "failed"
}

// DeepWorkflowCommand implements the full /workflow command.
type DeepWorkflowCommand struct{ BaseCommand }

func (c *DeepWorkflowCommand) Name() string                  { return "workflow" }
func (c *DeepWorkflowCommand) Aliases() []string             { return []string{"wf"} }
func (c *DeepWorkflowCommand) Description() string           { return "Manage and run workflows" }
func (c *DeepWorkflowCommand) ArgumentHint() string          { return "[list|run|create] [name]" }
func (c *DeepWorkflowCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepWorkflowCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepWorkflowCommand) ExecuteInteractive(_ context.Context, args []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &WorkflowViewData{Subcommand: "list"}

	if len(args) > 0 {
		data.Subcommand = strings.ToLower(args[0])
	}

	workDir := "."
	if ectx != nil && ectx.WorkDir != "" {
		workDir = ectx.WorkDir
	}

	switch data.Subcommand {
	case "list", "":
		data.Subcommand = "list"
		data.Workflows = discoverWorkflows(workDir)
		if len(data.Workflows) == 0 {
			data.Message = "No workflows found. Create one with /workflow create <name>"
		}

	case "run":
		if len(args) < 2 {
			data.Error = "Usage: /workflow run <name>"
		} else {
			name := args[1]
			wfs := discoverWorkflows(workDir)
			found := false
			for _, wf := range wfs {
				if wf.Name == name {
					found = true
					data.Active = &WorkflowRunStatus{
						Name:        wf.Name,
						CurrentStep: 0,
						TotalSteps:  wf.StepCount,
						Status:      "running",
					}
					break
				}
			}
			if !found {
				data.Error = fmt.Sprintf("Workflow '%s' not found", name)
				data.Workflows = wfs
			}
		}

	case "create":
		if len(args) < 2 {
			data.Error = "Usage: /workflow create <name>"
		} else {
			name := args[1]
			dir := filepath.Join(workDir, ".claude", "workflows")
			filePath := filepath.Join(dir, name+".md")

			if _, err := os.Stat(filePath); err == nil {
				data.Error = fmt.Sprintf("Workflow '%s' already exists at %s", name, filePath)
			} else {
				if err := os.MkdirAll(dir, 0755); err != nil {
					data.Error = fmt.Sprintf("Failed to create directory: %v", err)
				} else {
					content := fmt.Sprintf("---\ndescription: %s workflow\n---\n\n# %s\n\n1. Step one\n2. Step two\n", name, name)
					if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
						data.Error = fmt.Sprintf("Failed to create workflow: %v", err)
					} else {
						data.Message = fmt.Sprintf("Created workflow at %s", filePath)
						data.Workflows = discoverWorkflows(workDir)
					}
				}
			}
		}

	case "edit":
		if len(args) < 2 {
			data.Error = "Usage: /workflow edit <name>"
		} else {
			name := args[1]
			wfs := discoverWorkflows(workDir)
			for _, wf := range wfs {
				if wf.Name == name {
					data.Message = fmt.Sprintf("Edit workflow at: %s", wf.FilePath)
					break
				}
			}
			data.Workflows = wfs
		}

	case "delete":
		if len(args) < 2 {
			data.Error = "Usage: /workflow delete <name>"
		} else {
			name := args[1]
			dir := filepath.Join(workDir, ".claude", "workflows")
			filePath := filepath.Join(dir, name+".md")
			if err := os.Remove(filePath); err != nil {
				data.Error = fmt.Sprintf("Failed to delete workflow: %v", err)
			} else {
				data.Message = fmt.Sprintf("Deleted workflow '%s'", name)
				data.Workflows = discoverWorkflows(workDir)
			}
		}

	default:
		data.Subcommand = "list"
		data.Workflows = discoverWorkflows(workDir)
	}

	return &InteractiveResult{
		Component: "workflow",
		Data:      data,
	}, nil
}

// discoverWorkflows scans .claude/workflows/ for .md files.
func discoverWorkflows(workDir string) []WorkflowEntry {
	dir := filepath.Join(workDir, ".claude", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var workflows []WorkflowEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		filePath := filepath.Join(dir, entry.Name())

		desc, steps := parseWorkflowFile(filePath)
		workflows = append(workflows, WorkflowEntry{
			Name:        name,
			Description: desc,
			FilePath:    filePath,
			StepCount:   steps,
		})
	}

	return workflows
}

// parseWorkflowFile reads a workflow .md file and extracts description and step count.
func parseWorkflowFile(path string) (description string, stepCount int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0
	}

	content := string(data)

	// Parse YAML frontmatter.
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content[3:], "---", 2)
		if len(parts) >= 2 {
			frontmatter := parts[0]
			content = parts[1]

			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "description:") {
					description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
				}
			}
		}
	}

	// Count numbered steps (lines starting with a digit followed by a period).
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 1 && trimmed[0] >= '1' && trimmed[0] <= '9' && strings.Contains(trimmed[:min(4, len(trimmed))], ".") {
			stepCount++
		}
	}

	return description, stepCount
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	defaultRegistry.RegisterOrReplace(&DeepWorkflowCommand{})
}
