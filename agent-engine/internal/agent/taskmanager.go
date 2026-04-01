package agent

import (
	"fmt"
	"sync"
	"time"
)

// TaskManager tracks the lifecycle of all sub-agent tasks using a sync.Map.
type TaskManager struct {
	tasks sync.Map // agentID → *AgentTask
}

// NewTaskManager creates an empty TaskManager.
func NewTaskManager() *TaskManager { return &TaskManager{} }

// Create registers a new task in Pending state.
func (tm *TaskManager) Create(def AgentDefinition) *AgentTask {
	task := &AgentTask{
		Definition: def,
		Status:     AgentStatusPending,
	}
	tm.tasks.Store(def.AgentID, task)
	return task
}

// MarkRunning transitions a task to Running.
func (tm *TaskManager) MarkRunning(agentID string) error {
	return tm.update(agentID, func(t *AgentTask) {
		t.Status = AgentStatusRunning
		t.StartedAt = time.Now()
	})
}

// MarkDone transitions a task to Done with the given output.
func (tm *TaskManager) MarkDone(agentID, output string) error {
	return tm.update(agentID, func(t *AgentTask) {
		t.Status = AgentStatusDone
		t.Output = output
		t.FinishedAt = time.Now()
	})
}

// MarkFailed transitions a task to Failed with an error message.
func (tm *TaskManager) MarkFailed(agentID, errMsg string) error {
	return tm.update(agentID, func(t *AgentTask) {
		t.Status = AgentStatusFailed
		t.Error = errMsg
		t.FinishedAt = time.Now()
	})
}

// MarkCancelled transitions a task to Cancelled.
func (tm *TaskManager) MarkCancelled(agentID string) error {
	return tm.update(agentID, func(t *AgentTask) {
		t.Status = AgentStatusCancelled
		t.FinishedAt = time.Now()
	})
}

// Get returns the task for agentID.
func (tm *TaskManager) Get(agentID string) (*AgentTask, bool) {
	v, ok := tm.tasks.Load(agentID)
	if !ok {
		return nil, false
	}
	return v.(*AgentTask), true
}

// All returns a snapshot of all tasks.
func (tm *TaskManager) All() []*AgentTask {
	var tasks []*AgentTask
	tm.tasks.Range(func(_, v interface{}) bool {
		tasks = append(tasks, v.(*AgentTask))
		return true
	})
	return tasks
}

// Active returns only tasks currently in Running or Pending state.
func (tm *TaskManager) Active() []*AgentTask {
	var active []*AgentTask
	tm.tasks.Range(func(_, v interface{}) bool {
		t := v.(*AgentTask)
		if t.Status == AgentStatusRunning || t.Status == AgentStatusPending {
			active = append(active, t)
		}
		return true
	})
	return active
}

// Delete removes a task from the manager.
func (tm *TaskManager) Delete(agentID string) {
	tm.tasks.Delete(agentID)
}

func (tm *TaskManager) update(agentID string, fn func(*AgentTask)) error {
	v, ok := tm.tasks.Load(agentID)
	if !ok {
		return fmt.Errorf("agent task %q not found", agentID)
	}
	fn(v.(*AgentTask))
	return nil
}
