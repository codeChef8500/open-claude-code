package lsptool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// LSPProvider is the interface for interacting with Language Server Protocol servers.
// The actual implementation connects to an IDE's LSP bridge or a standalone LSP server.
type LSPProvider interface {
	// GetDiagnostics returns diagnostics (errors, warnings) for a file.
	GetDiagnostics(ctx context.Context, filePath string) ([]Diagnostic, error)
	// GetHover returns hover information at a specific position.
	GetHover(ctx context.Context, filePath string, line, col int) (string, error)
	// GetDefinition returns the location of a symbol's definition.
	GetDefinition(ctx context.Context, filePath string, line, col int) ([]Location, error)
	// GetReferences returns all references to a symbol.
	GetReferences(ctx context.Context, filePath string, line, col int) ([]Location, error)
	// IsAvailable returns true if the LSP server is connected and ready.
	IsAvailable() bool
}

// Diagnostic represents an LSP diagnostic.
type Diagnostic struct {
	FilePath string `json:"filePath"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Severity string `json:"severity"` // "error", "warning", "info", "hint"
	Message  string `json:"message"`
	Source   string `json:"source,omitempty"`
}

// Location represents a source code location.
type Location struct {
	FilePath string `json:"filePath"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	EndLine  int    `json:"endLine,omitempty"`
	EndCol   int    `json:"endCol,omitempty"`
}

// LSPTool provides Language Server Protocol integration for code intelligence.
type LSPTool struct {
	tool.BaseTool
	provider LSPProvider
}

// New creates an LSPTool with the given provider.
func New(provider LSPProvider) *LSPTool {
	return &LSPTool{provider: provider}
}

func (t *LSPTool) Name() string           { return "lsp" }
func (t *LSPTool) UserFacingName() string { return "LSP" }
func (t *LSPTool) Description() string {
	return "Access Language Server Protocol features: diagnostics, hover info, definitions, and references."
}
func (t *LSPTool) MaxResultSizeChars() int                  { return 100_000 }
func (t *LSPTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (t *LSPTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *LSPTool) ShouldDefer() bool                        { return true }
func (t *LSPTool) IsLSP() bool                              { return true }
func (t *LSPTool) SearchHint() string {
	return "get diagnostics, definitions, references from language server"
}

func (t *LSPTool) IsEnabled(_ *tool.UseContext) bool {
	if t.provider == nil {
		return false
	}
	return t.provider.IsAvailable()
}

func (t *LSPTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["diagnostics", "hover", "definition", "references"],
				"description": "The LSP action to perform."
			},
			"filePath": {
				"type": "string",
				"description": "The file path to query."
			},
			"line": {
				"type": "integer",
				"description": "1-based line number (required for hover, definition, references)."
			},
			"col": {
				"type": "integer",
				"description": "1-based column number (required for hover, definition, references)."
			}
		},
		"required": ["action", "filePath"]
	}`)
}

func (t *LSPTool) Prompt(_ *tool.UseContext) string {
	return `Interact with Language Server Protocol servers for code intelligence operations.

Available operations:
- goToDefinition: Jump to where a symbol is defined
- findReferences: Find all references to a symbol
- hover: Get type/documentation info for a symbol at a position
- documentSymbol: List all symbols in a document
- workspaceSymbol: Search for symbols across the workspace

Usage:
- Provide file_path and position (line/character, both 0-indexed) for position-based operations
- Use this tool for precise code navigation instead of text-based grep when an LSP server is available
- This tool is read-only and concurrent-safe
- Requires an active LSP server connection`
}

func (t *LSPTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil
}

func (t *LSPTool) Call(ctx context.Context, input json.RawMessage, _ *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	ch := make(chan *engine.ContentBlock, 2)

	go func() {
		defer close(ch)

		var args struct {
			Action   string `json:"action"`
			FilePath string `json:"filePath"`
			Line     int    `json:"line"`
			Col      int    `json:"col"`
		}
		if err := json.Unmarshal(input, &args); err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "invalid input: " + err.Error(), IsError: true}
			return
		}

		if t.provider == nil || !t.provider.IsAvailable() {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    "LSP server is not available. Ensure an IDE with LSP support is connected.",
				IsError: true,
			}
			return
		}

		switch args.Action {
		case "diagnostics":
			diags, err := t.provider.GetDiagnostics(ctx, args.FilePath)
			if err != nil {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "diagnostics error: " + err.Error(), IsError: true}
				return
			}
			if len(diags) == 0 {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "No diagnostics found."}
				return
			}
			data, _ := json.MarshalIndent(diags, "", "  ")
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(data)}

		case "hover":
			if args.Line == 0 || args.Col == 0 {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "line and col are required for hover", IsError: true}
				return
			}
			info, err := t.provider.GetHover(ctx, args.FilePath, args.Line, args.Col)
			if err != nil {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "hover error: " + err.Error(), IsError: true}
				return
			}
			if info == "" {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "No hover information available at this position."}
				return
			}
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: info}

		case "definition":
			if args.Line == 0 || args.Col == 0 {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "line and col are required for definition", IsError: true}
				return
			}
			locs, err := t.provider.GetDefinition(ctx, args.FilePath, args.Line, args.Col)
			if err != nil {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "definition error: " + err.Error(), IsError: true}
				return
			}
			if len(locs) == 0 {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "No definition found."}
				return
			}
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: formatLocations(locs)}

		case "references":
			if args.Line == 0 || args.Col == 0 {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "line and col are required for references", IsError: true}
				return
			}
			locs, err := t.provider.GetReferences(ctx, args.FilePath, args.Line, args.Col)
			if err != nil {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "references error: " + err.Error(), IsError: true}
				return
			}
			if len(locs) == 0 {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: "No references found."}
				return
			}
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: formatLocations(locs)}

		default:
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    fmt.Sprintf("Unknown action %q. Use: diagnostics, hover, definition, references.", args.Action),
				IsError: true,
			}
		}
	}()

	return ch, nil
}

func formatLocations(locs []Location) string {
	var lines []string
	for _, loc := range locs {
		lines = append(lines, fmt.Sprintf("%s:%d:%d", loc.FilePath, loc.Line, loc.Col))
	}
	return strings.Join(lines, "\n")
}
