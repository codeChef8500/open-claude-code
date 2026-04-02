package permission

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// AuditEntry records a single permission decision.
type AuditEntry struct {
	Timestamp  time.Time      `json:"timestamp"`
	SessionID  string         `json:"session_id,omitempty"`
	ToolName   string         `json:"tool_name"`
	Input      string         `json:"input,omitempty"`
	Decision   AuditDecision  `json:"decision"`
	Reason     string         `json:"reason,omitempty"`
	RuleSource string         `json:"rule_source,omitempty"`
}

// AuditDecision is the outcome of a permission check.
type AuditDecision string

const (
	AuditAllow AuditDecision = "allow"
	AuditDeny  AuditDecision = "deny"
	AuditAsk   AuditDecision = "ask"
)

// AuditLog records permission decisions to an append-only JSONL file.
type AuditLog struct {
	mu   sync.Mutex
	file *os.File
}

// NewAuditLog opens (or creates) a JSONL audit log at the given path.
func NewAuditLog(path string) (*AuditLog, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("permission audit log: open %q: %w", path, err)
	}
	return &AuditLog{file: f}, nil
}

// Record appends a permission decision to the log.
func (a *AuditLog) Record(entry AuditEntry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	_, _ = fmt.Fprintf(a.file, "%s\n", b)
}

// Close flushes and closes the underlying file.
func (a *AuditLog) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.file.Close()
}

// NopAuditLog is a no-op AuditLog used when auditing is disabled.
type NopAuditLog struct{}

func (NopAuditLog) Record(_ AuditEntry) {}
func (NopAuditLog) Close() error        { return nil }

// AuditRecorder is the common interface for audit log implementations.
type AuditRecorder interface {
	Record(AuditEntry)
	Close() error
}

// DenyReason classifies why a permission check was denied.
type DenyReason string

const (
	DenyReasonNoRule         DenyReason = "no_matching_rule"
	DenyReasonDangerousShell DenyReason = "dangerous_shell_pattern"
	DenyReasonFailClosed     DenyReason = "fail_closed"
	DenyReasonUserDenied     DenyReason = "user_denied"
	DenyReasonPathBlocked    DenyReason = "path_blocked"
	DenyReasonRuleMatch      DenyReason = "explicit_deny_rule"
)

// FormatDenyMessage returns a human-readable denial message.
func FormatDenyMessage(toolName, input string, reason DenyReason) string {
	switch reason {
	case DenyReasonDangerousShell:
		return fmt.Sprintf("Permission denied: dangerous shell pattern detected in %q for tool %q", input, toolName)
	case DenyReasonPathBlocked:
		return fmt.Sprintf("Permission denied: path %q is in a blocked system directory (tool: %q)", input, toolName)
	case DenyReasonUserDenied:
		return fmt.Sprintf("Permission denied: user explicitly denied access for tool %q", toolName)
	case DenyReasonFailClosed:
		return fmt.Sprintf("Permission denied: fail-closed mode active for tool %q", toolName)
	case DenyReasonRuleMatch:
		return fmt.Sprintf("Permission denied: explicit deny rule matched for tool %q input %q", toolName, input)
	default:
		return fmt.Sprintf("Permission denied: no matching allow rule for tool %q input %q", toolName, input)
	}
}
