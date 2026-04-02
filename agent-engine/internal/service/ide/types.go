package ide

// DiagnosticSeverity mirrors LSP DiagnosticSeverity values.
type DiagnosticSeverity int

const (
	SeverityError       DiagnosticSeverity = 1
	SeverityWarning     DiagnosticSeverity = 2
	SeverityInformation DiagnosticSeverity = 3
	SeverityHint        DiagnosticSeverity = 4
)

// Position is a zero-based line/character position (LSP convention).
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is a start/end position pair.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Diagnostic is a single LSP-compatible diagnostic message.
type Diagnostic struct {
	Range    Range              `json:"range"`
	Severity DiagnosticSeverity `json:"severity"`
	Code     string             `json:"code,omitempty"`
	Source   string             `json:"source,omitempty"`
	Message  string             `json:"message"`
}

// TextEdit represents a single text replacement in a file.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// WorkspaceEdit groups file-level edits keyed by URI.
type WorkspaceEdit struct {
	// Changes maps file URI → slice of TextEdits.
	Changes map[string][]TextEdit `json:"changes,omitempty"`
}

// FileEvent represents a file change notification from the IDE.
type FileEvent struct {
	URI  string      `json:"uri"`
	Type FileChangeType `json:"type"`
}

// FileChangeType mirrors LSP FileChangeType.
type FileChangeType int

const (
	FileCreated FileChangeType = 1
	FileChanged FileChangeType = 2
	FileDeleted FileChangeType = 3
)

// IDECapabilities describes what the connected IDE supports.
type IDECapabilities struct {
	// SupportsInlayHints indicates the IDE can render inlay hints.
	SupportsInlayHints bool `json:"supports_inlay_hints"`
	// SupportsCodeLens indicates CodeLens support.
	SupportsCodeLens bool `json:"supports_code_lens"`
	// SupportsSemanticTokens indicates semantic token support.
	SupportsSemanticTokens bool `json:"supports_semantic_tokens"`
	// IDEName is a human-readable IDE identifier, e.g. "vscode", "neovim".
	IDEName string `json:"ide_name,omitempty"`
	// IDEVersion is the IDE version string.
	IDEVersion string `json:"ide_version,omitempty"`
}

// IDERequest is a request from the IDE to the agent engine.
type IDERequest struct {
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

// IDEResponse is a response sent back to the IDE.
type IDEResponse struct {
	ID     interface{} `json:"id,omitempty"`
	Result interface{} `json:"result,omitempty"`
	Error  *IDEError   `json:"error,omitempty"`
}

// IDEError wraps an error for the IDE wire protocol.
type IDEError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
