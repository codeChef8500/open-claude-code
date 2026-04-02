package permission

// Mode represents the overall permission policy.
type Mode string

const (
	ModeDefault     Mode = "default"           // Ask for sensitive operations
	ModeAutoApprove Mode = "auto"              // Auto Mode (LLM classifier)
	ModeBypassAll   Mode = "bypassPermissions" // Bypass all checks (dangerous)
	ModePlan        Mode = "plan"              // Plan mode (read-only, no writes)
	ModeAcceptEdits Mode = "acceptEdits"       // Accept file edits without asking
	ModeDontAsk     Mode = "dontAsk"           // Never ask — deny if not auto-allowed
	ModeBubble      Mode = "bubble"            // Bubble up to parent agent
)

// InternalPermissionModes are modes used internally (not user-selectable).
var InternalPermissionModes = []Mode{ModeBubble}

// ExternalPermissionModes are the modes exposed to the user in UI/CLI.
var ExternalPermissionModes = []Mode{
	ModeDefault, ModeAutoApprove, ModeBypassAll,
	ModePlan, ModeAcceptEdits, ModeDontAsk,
}

// Result is the outcome of a permission check.
type Result int

const (
	ResultAllow    Result = 0 // Permitted immediately
	ResultDeny     Result = 1 // Denied immediately
	ResultAsk      Result = 2 // User must confirm
	ResultSoftDeny Result = 3 // Auto Mode soft-deny (can retry with explicit allow)
)

// Behavior maps to the action taken for a matched permission rule.
type Behavior string

const (
	BehaviorAllow Behavior = "allow" // Permit without asking
	BehaviorDeny  Behavior = "deny"  // Block unconditionally
	BehaviorAsk   Behavior = "ask"   // Ask the user
)

// CheckRequest contains everything needed to evaluate a permission.
type CheckRequest struct {
	ToolName  string
	ToolInput interface{}
	WorkDir   string
	AgentID   string
	Mode      Mode

	// AdditionalWorkingDirs are extra directories the session may access.
	AdditionalWorkingDirs []string
	// IsReadOnly is true if the tool is a read-only operation.
	IsReadOnly bool
	// IsConcurrencySafe is true if the tool is safe to run concurrently.
	IsConcurrencySafe bool
	// IsDestructive is true if the tool makes irreversible changes.
	IsDestructive bool
	// IsPlanMode is true when the session is in plan-only mode.
	IsPlanMode bool
	// Filepath is the path the tool will operate on (for filesystem checks).
	Filepath string
	// Command is the shell command (for BashTool checks).
	Command string
}

// RuleType classifies a permission rule.
type RuleType string

const (
	RuleAllow RuleType = "allow"
	RuleDeny  RuleType = "deny"
	RuleAsk   RuleType = "ask"
)

// RuleSource indicates where a permission rule was defined.
type RuleSource string

const (
	RuleSourceUserSettings    RuleSource = "user_settings"
	RuleSourceProjectSettings RuleSource = "project_settings"
	RuleSourceLocalSettings   RuleSource = "local_settings"
	RuleSourceFlagSettings    RuleSource = "flag_settings"
	RuleSourcePolicySettings  RuleSource = "policy_settings"
	RuleSourceCLIArg          RuleSource = "cli_arg"
	RuleSourceCommand         RuleSource = "command"
	RuleSourceSession         RuleSource = "session"
)

// Rule is a single permission rule entry.
type Rule struct {
	Type     RuleType   `json:"type"`
	Pattern  string     `json:"pattern"`             // glob or exact match
	ToolName string     `json:"tool_name,omitempty"` // empty = applies to all tools
	Source   RuleSource `json:"source,omitempty"`
}

// ToolPermissionContext carries the complete permission evaluation context
// for a given session. This is the Go equivalent of claude-code-main's
// ToolPermissionContext type.
type ToolPermissionContext struct {
	// Mode is the active permission mode.
	Mode Mode `json:"mode"`

	// IsBypassAvailable indicates whether bypass mode is allowed (set by policy).
	IsBypassAvailable bool `json:"is_bypass_available"`

	// Rules grouped by source.
	RulesBySource map[RuleSource][]Rule `json:"rules_by_source,omitempty"`

	// SessionRules are rules added during the current session (e.g. user approvals).
	SessionRules []Rule `json:"session_rules,omitempty"`

	// DenialCount tracks consecutive permission denials.
	DenialCount int `json:"denial_count,omitempty"`

	// FailClosed is true when too many denials triggered fail-closed mode.
	FailClosed bool `json:"fail_closed,omitempty"`
}

// AllRules returns all rules from all sources, flattened.
func (c *ToolPermissionContext) AllRules() []Rule {
	var all []Rule
	for _, rules := range c.RulesBySource {
		all = append(all, rules...)
	}
	all = append(all, c.SessionRules...)
	return all
}

// PermissionUpdate represents a runtime change to permission configuration.
type PermissionUpdate struct {
	// Destination specifies where the update should be persisted.
	Destination RuleSource `json:"destination"`

	// Rule is the new rule to add or remove.
	Rule Rule `json:"rule"`

	// Remove, if true, removes a matching rule instead of adding one.
	Remove bool `json:"remove,omitempty"`
}

// DenialRecord stores a single permission denial event for audit / diagnostics.
type DenialRecord struct {
	ToolName  string      `json:"tool_name"`
	Reason    string      `json:"reason"`
	Input     interface{} `json:"input,omitempty"`
	RuleType  string      `json:"rule_type,omitempty"` // "auto_classifier", "rule_based", "hook", "user_denied"
	Timestamp int64       `json:"timestamp,omitempty"` // unix millis
	AgentID   string      `json:"agent_id,omitempty"`
}

// DangerousShellPatterns are shell fragments that are always denied regardless
// of other allow rules.  These cover the most destructive one-liners.
var DangerousShellPatterns = []string{
	"rm -rf /",
	"rm -rf ~",
	":(){ :|:& };:", // fork bomb
	"dd if=/dev/zero of=/dev/",
	"> /dev/sda",
	"mkfs",
	"format c:",
	"del /f /s /q c:\\",
	"shutdown",
	"reboot",
	"halt",
	"poweroff",
}
