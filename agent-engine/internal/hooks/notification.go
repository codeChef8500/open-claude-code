package hooks

import (
	"context"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// Notification hook helpers — convenience wrappers for change notification
// events (file changes, config changes, cwd changes).
// Aligned with claude-code-main hooks.ts notification patterns.
// ────────────────────────────────────────────────────────────────────────────

// NotifyFileChanged fires the FileChanged hook when a file is created, modified,
// or deleted. changeType should be one of "created", "modified", "deleted".
func NotifyFileChanged(ctx context.Context, exec *Executor, reg *Registry, filePath, changeType string) {
	input := &HookInput{
		Timestamp: time.Now(),
		FileChanged: &FileChangedInput{
			FilePath:   filePath,
			ChangeType: changeType,
		},
	}
	if exec != nil {
		exec.RunAsync(EventFileChanged, input)
	}
	if reg != nil {
		reg.RunAsync(EventFileChanged, input)
	}
}

// NotifyConfigChange fires the ConfigChange hook when a configuration value changes.
func NotifyConfigChange(ctx context.Context, exec *Executor, reg *Registry, key, oldValue, newValue string) {
	input := &HookInput{
		Timestamp: time.Now(),
		ConfigChange: &ConfigChangeInput{
			Key:      key,
			OldValue: oldValue,
			NewValue: newValue,
		},
	}
	if exec != nil {
		exec.RunAsync(EventConfigChange, input)
	}
	if reg != nil {
		reg.RunAsync(EventConfigChange, input)
	}
}

// NotifyCwdChanged fires the CwdChanged hook when the working directory changes.
func NotifyCwdChanged(ctx context.Context, exec *Executor, reg *Registry, newCwd string) {
	input := &HookInput{
		Timestamp: time.Now(),
		ConfigChange: &ConfigChangeInput{
			Key:      "cwd",
			NewValue: newCwd,
		},
	}
	if exec != nil {
		exec.RunAsync(EventCwdChanged, input)
	}
	if reg != nil {
		reg.RunAsync(EventCwdChanged, input)
	}
}

// NotifyMessage fires the Notification hook with a message string.
func NotifyMessage(ctx context.Context, exec *Executor, reg *Registry, message string) {
	input := &HookInput{
		Timestamp: time.Now(),
		Notification: &NotificationInput{
			Message: message,
		},
	}
	if exec != nil {
		exec.RunAsync(EventNotification, input)
	}
	if reg != nil {
		reg.RunAsync(EventNotification, input)
	}
}
