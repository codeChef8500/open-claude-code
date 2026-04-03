package hooks

import (
	"context"
	"encoding/json"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// Lifecycle hook helpers — convenience wrappers for common lifecycle events.
// Aligned with claude-code-main hooks.ts lifecycle hook invocations.
// ────────────────────────────────────────────────────────────────────────────

// RunSessionStart fires the SessionStart hook.
func RunSessionStart(ctx context.Context, exec *Executor, reg *Registry) SyncHookResponse {
	input := &HookInput{Timestamp: time.Now()}
	resp := SyncHookResponse{}
	if exec != nil {
		resp = exec.RunAll(ctx, EventSessionStart, input)
	}
	if reg != nil {
		rResp := reg.RunSync(ctx, EventSessionStart, input)
		reg.RunAsync(EventSessionStart, input)
		mergeResponse(&resp, &rResp)
	}
	return resp
}

// RunSessionEnd fires the SessionEnd hook.
func RunSessionEnd(ctx context.Context, exec *Executor, reg *Registry) {
	input := &HookInput{Timestamp: time.Now()}
	if exec != nil {
		exec.RunAll(ctx, EventSessionEnd, input)
	}
	if reg != nil {
		reg.RunSync(ctx, EventSessionEnd, input)
		reg.RunAsync(EventSessionEnd, input)
	}
}

// RunSetup fires the Setup hook at application startup.
func RunSetup(ctx context.Context, exec *Executor, reg *Registry) SyncHookResponse {
	input := &HookInput{Timestamp: time.Now()}
	resp := SyncHookResponse{}
	if exec != nil {
		resp = exec.RunAll(ctx, EventSetup, input)
	}
	if reg != nil {
		rResp := reg.RunSync(ctx, EventSetup, input)
		reg.RunAsync(EventSetup, input)
		mergeResponse(&resp, &rResp)
	}
	return resp
}

// RunSubagentStart fires the SubagentStart hook when a sub-agent is created.
func RunSubagentStart(ctx context.Context, exec *Executor, reg *Registry, taskID, title string) {
	input := &HookInput{
		Timestamp: time.Now(),
		Task: &TaskInput{
			TaskID: taskID,
			Title:  title,
			Status: "started",
		},
	}
	if exec != nil {
		exec.RunAsync(EventSubagentStart, input)
	}
	if reg != nil {
		reg.RunAsync(EventSubagentStart, input)
	}
}

// RunSubagentStop fires the SubagentStop hook when a sub-agent finishes.
func RunSubagentStop(ctx context.Context, exec *Executor, reg *Registry, taskID, status string) {
	input := &HookInput{
		Timestamp: time.Now(),
		Task: &TaskInput{
			TaskID: taskID,
			Status: status,
		},
	}
	if exec != nil {
		exec.RunAsync(EventSubagentStop, input)
	}
	if reg != nil {
		reg.RunAsync(EventSubagentStop, input)
	}
}

// RunTaskCreated fires the TaskCreated hook.
func RunTaskCreated(ctx context.Context, exec *Executor, reg *Registry, taskID, title, description string) {
	input := &HookInput{
		Timestamp: time.Now(),
		Task: &TaskInput{
			TaskID:      taskID,
			Title:       title,
			Description: description,
			Status:      "created",
		},
	}
	if exec != nil {
		exec.RunAsync(EventTaskCreated, input)
	}
	if reg != nil {
		reg.RunAsync(EventTaskCreated, input)
	}
}

// RunTaskCompleted fires the TaskCompleted hook.
func RunTaskCompleted(ctx context.Context, exec *Executor, reg *Registry, taskID, status string) {
	input := &HookInput{
		Timestamp: time.Now(),
		Task: &TaskInput{
			TaskID: taskID,
			Status: status,
		},
	}
	if exec != nil {
		exec.RunAsync(EventTaskCompleted, input)
	}
	if reg != nil {
		reg.RunAsync(EventTaskCompleted, input)
	}
}

// RunTeammateIdle fires the TeammateIdle hook when a teammate has no work.
func RunTeammateIdle(ctx context.Context, exec *Executor, reg *Registry, taskID string) {
	input := &HookInput{
		Timestamp: time.Now(),
		Task: &TaskInput{
			TaskID: taskID,
			Status: "idle",
		},
	}
	if exec != nil {
		exec.RunAsync(EventTeammateIdle, input)
	}
	if reg != nil {
		reg.RunAsync(EventTeammateIdle, input)
	}
}

// RunPreToolUse fires the PreToolUse hook and returns the merged response.
func RunPreToolUse(ctx context.Context, exec *Executor, reg *Registry, toolName, toolID string, toolInput json.RawMessage) SyncHookResponse {
	input := &HookInput{
		Timestamp: time.Now(),
		PreToolUse: &PreToolUseInput{
			ToolName: toolName,
			ToolID:   toolID,
			Input:    toolInput,
		},
	}
	resp := SyncHookResponse{}
	if exec != nil {
		resp = exec.RunSync(ctx, EventPreToolUse, input)
	}
	if reg != nil {
		rResp := reg.RunSync(ctx, EventPreToolUse, input)
		mergeResponse(&resp, &rResp)
	}
	return resp
}

// RunPostToolUse fires the PostToolUse hook and returns the merged response.
func RunPostToolUse(ctx context.Context, exec *Executor, reg *Registry, toolName, toolID string, toolInput json.RawMessage, output string, isError bool) SyncHookResponse {
	input := &HookInput{
		Timestamp: time.Now(),
		PostToolUse: &PostToolUseInput{
			ToolName: toolName,
			ToolID:   toolID,
			Input:    toolInput,
			Output:   output,
			IsError:  isError,
		},
	}
	resp := SyncHookResponse{}
	if exec != nil {
		resp = exec.RunSync(ctx, EventPostToolUse, input)
	}
	if reg != nil {
		rResp := reg.RunSync(ctx, EventPostToolUse, input)
		mergeResponse(&resp, &rResp)
	}
	return resp
}

// RunStop fires the Stop hook and returns the merged response.
func RunStop(ctx context.Context, exec *Executor, reg *Registry, stopReason, assistantMessage string) SyncHookResponse {
	input := &HookInput{
		Timestamp: time.Now(),
		Stop: &StopInput{
			StopReason:       stopReason,
			AssistantMessage: assistantMessage,
		},
	}
	resp := SyncHookResponse{}
	if exec != nil {
		resp = exec.RunSync(ctx, EventStop, input)
	}
	if reg != nil {
		rResp := reg.RunSync(ctx, EventStop, input)
		mergeResponse(&resp, &rResp)
	}
	return resp
}

// RunPostSampling fires the PostSampling hook asynchronously.
func RunPostSampling(ctx context.Context, exec *Executor, reg *Registry, content json.RawMessage, stopReason string) {
	input := &HookInput{
		Timestamp: time.Now(),
		PostSampling: &PostSamplingInput{
			AssistantContent: content,
			StopReason:       stopReason,
		},
	}
	if exec != nil {
		exec.RunAsync(EventPostSampling, input)
	}
	if reg != nil {
		reg.RunAsync(EventPostSampling, input)
	}
}

// RunPreCompact fires the PreCompact hook.
func RunPreCompact(ctx context.Context, exec *Executor, reg *Registry, trigger, customInstructions string) SyncHookResponse {
	input := &HookInput{
		Timestamp: time.Now(),
		PreCompact: &PreCompactInput{
			Trigger:            trigger,
			CustomInstructions: customInstructions,
		},
	}
	resp := SyncHookResponse{}
	if exec != nil {
		resp = exec.RunSync(ctx, EventPreCompact, input)
	}
	if reg != nil {
		rResp := reg.RunSync(ctx, EventPreCompact, input)
		mergeResponse(&resp, &rResp)
	}
	return resp
}

// RunPostCompact fires the PostCompact hook asynchronously.
func RunPostCompact(ctx context.Context, exec *Executor, reg *Registry) {
	input := &HookInput{Timestamp: time.Now()}
	if exec != nil {
		exec.RunAsync(EventPostCompact, input)
	}
	if reg != nil {
		reg.RunAsync(EventPostCompact, input)
	}
}

// RunPermissionDenied fires the PermissionDenied hook asynchronously.
func RunPermissionDenied(ctx context.Context, exec *Executor, reg *Registry, toolName, reason, ruleType string) {
	input := &HookInput{
		Timestamp: time.Now(),
		PermissionDenied: &PermissionDeniedInput{
			ToolName: toolName,
			Reason:   reason,
			RuleType: ruleType,
		},
	}
	if exec != nil {
		exec.RunAsync(EventPermissionDenied, input)
	}
	if reg != nil {
		reg.RunAsync(EventPermissionDenied, input)
	}
}

// RunUserPromptSubmit fires the UserPromptSubmit hook.
func RunUserPromptSubmit(ctx context.Context, exec *Executor, reg *Registry) SyncHookResponse {
	input := &HookInput{Timestamp: time.Now()}
	resp := SyncHookResponse{}
	if exec != nil {
		resp = exec.RunSync(ctx, EventUserPromptSubmit, input)
	}
	if reg != nil {
		rResp := reg.RunSync(ctx, EventUserPromptSubmit, input)
		mergeResponse(&resp, &rResp)
	}
	return resp
}

// RunInstructionsLoaded fires the InstructionsLoaded hook asynchronously.
func RunInstructionsLoaded(ctx context.Context, exec *Executor, reg *Registry) {
	input := &HookInput{Timestamp: time.Now()}
	if exec != nil {
		exec.RunAsync(EventInstructionsLoaded, input)
	}
	if reg != nil {
		reg.RunAsync(EventInstructionsLoaded, input)
	}
}
