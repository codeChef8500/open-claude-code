package engine

// MemoryLoader loads CLAUDE.md memory content for a working directory.
// Implemented by the memory package; wired at SDK construction time to
// avoid an import cycle (memory → engine, engine → memory would cycle).
type MemoryLoader interface {
	LoadMemory(workDir string) (string, error)
}

// SessionWriter persists conversation messages to durable storage (JSONL).
// Implemented by the session package; wired at SDK construction time.
type SessionWriter interface {
	AppendMessage(sessionID string, msg *Message) error
}

// SystemPromptResult holds the built system prompt in both flat-string and
// segmented forms.  Providers that support prompt caching use Parts; others
// fall back to Text.
type SystemPromptResult struct {
	// Text is the full concatenated prompt (always populated).
	Text string
	// Parts holds the ordered cache-aware segments (may be nil).
	Parts []SystemPromptPart
}

// SystemPromptBuilder constructs the full multi-layer system prompt given the
// current engine state.  Implemented by the prompt package; wired at SDK
// construction time.
type SystemPromptBuilder interface {
	// BuildParts returns both the combined text and per-segment parts.
	BuildParts(opts SystemPromptOptions) SystemPromptResult
}

// SystemPromptOptions carries the inputs needed by SystemPromptBuilder.BuildParts.
type SystemPromptOptions struct {
	Tools              []Tool
	UseContext         *UseContext
	WorkDir            string
	MemoryContent      string
	CustomSystemPrompt string
	AppendSystemPrompt string
	KairosActive       bool   // inject KAIROS daemon mode instructions
	BuddyActive        bool   // inject companion intro into system prompt
	CompanionName      string // companion name (for intro text)
	CompanionSpecies   string // companion species (for intro text)
	AutoMemoryPrompt   string // auto-memory prompt (replaces MemoryContent when set)
	TeamMemoryEnabled  bool   // team memory is active
}

// PermissionVerdict is the outcome of a global permission check.
type PermissionVerdict int

const (
	PermissionAllow    PermissionVerdict = 0
	PermissionDeny     PermissionVerdict = 1
	PermissionSoftDeny PermissionVerdict = 2
)

// GlobalPermissionChecker runs a global policy check before any tool is called.
// Implemented by the permission package; wired at SDK construction time.
type GlobalPermissionChecker interface {
	// CheckTool returns the permission verdict and an explanatory reason.
	CheckTool(ctx interface{ Done() <-chan struct{} }, toolName string, toolInput interface{}, workDir string) (PermissionVerdict, string)
}

// AutoModeClassifier runs the LLM-based Auto Mode side-query for a tool call.
// Implemented by the mode package; wired at SDK construction time.
type AutoModeClassifier interface {
	// Classify returns allow/soft_deny/deny and a reason string.
	Classify(ctx interface{ Done() <-chan struct{} }, toolName string, toolInput interface{}) (PermissionVerdict, string, error)
}
