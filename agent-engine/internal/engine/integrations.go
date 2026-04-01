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

// SystemPromptBuilder constructs the full multi-layer system prompt string
// given the current engine state.  Implemented by the prompt package; wired
// at SDK construction time.
type SystemPromptBuilder interface {
	// Build returns the combined system prompt text.
	Build(opts SystemPromptOptions) string
}

// SystemPromptOptions carries the inputs needed by SystemPromptBuilder.Build.
type SystemPromptOptions struct {
	Tools              []Tool
	UseContext         *UseContext
	WorkDir            string
	MemoryContent      string
	CustomSystemPrompt string
	AppendSystemPrompt string
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
