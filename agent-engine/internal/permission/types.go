package permission

// Mode represents the overall permission policy.
type Mode string

const (
	ModeDefault     Mode = "default" // Ask for sensitive operations
	ModeAutoApprove Mode = "auto"    // Auto Mode (LLM classifier)
	ModeBypassAll   Mode = "bypass"  // Bypass all checks (dangerous)
)

// Result is the outcome of a permission check.
type Result int

const (
	ResultAllow    Result = 0 // Permitted immediately
	ResultDeny     Result = 1 // Denied immediately
	ResultAsk      Result = 2 // User must confirm
	ResultSoftDeny Result = 3 // Auto Mode soft-deny (can retry with explicit allow)
)

// CheckRequest contains everything needed to evaluate a permission.
type CheckRequest struct {
	ToolName  string
	ToolInput interface{}
	WorkDir   string
	AgentID   string
	Mode      Mode
}

// RuleType classifies a permission rule.
type RuleType string

const (
	RuleAllow RuleType = "allow"
	RuleDeny  RuleType = "deny"
)

// Rule is a single allowlist or denylist entry.
type Rule struct {
	Type    RuleType
	Pattern string // glob or exact match
}

// DenialRecord stores a single permission denial event for audit / diagnostics.
type DenialRecord struct {
	ToolName string
	Reason   string
	Input    interface{}
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
