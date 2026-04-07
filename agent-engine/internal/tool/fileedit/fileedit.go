package fileedit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
	"github.com/wall-ai/agent-engine/internal/tool/diff"
	"github.com/wall-ai/agent-engine/internal/tool/fileread"
	"github.com/wall-ai/agent-engine/internal/util"
)

type Input struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

// StructuredPatchHunk represents a single diff hunk, matching claude-code-main's format.
type StructuredPatchHunk struct {
	OldStart int      `json:"oldStart"`
	OldLines int      `json:"oldLines"`
	NewStart int      `json:"newStart"`
	NewLines int      `json:"newLines"`
	Lines    []string `json:"lines"`
}

// Output is the structured output of a FileEdit call.
type Output struct {
	FilePath        string                `json:"filePath"`
	OldString       string                `json:"oldString"`
	NewString       string                `json:"newString"`
	OriginalFile    string                `json:"originalFile"`
	StructuredPatch []StructuredPatchHunk `json:"structuredPatch"`
	ReplaceAll      bool                  `json:"replaceAll"`
}

type FileEditTool struct{ tool.BaseTool }

func New() *FileEditTool { return &FileEditTool{} }

func (t *FileEditTool) Name() string                             { return "Edit" }
func (t *FileEditTool) Description() string                      { return "Replace an exact string in a file." }
func (t *FileEditTool) IsReadOnly(_ json.RawMessage) bool        { return false }
func (t *FileEditTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (t *FileEditTool) MaxResultSizeChars() int                  { return 0 }
func (t *FileEditTool) IsEnabled(_ *tool.UseContext) bool        { return true }
func (t *FileEditTool) IsDestructive(_ json.RawMessage) bool     { return true }
func (t *FileEditTool) ShouldDefer() bool                        { return true }
func (t *FileEditTool) GetPath(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return ""
	}
	return in.FilePath
}

func (t *FileEditTool) UserFacingName() string { return "Update" }

// DynamicUserFacingName returns "Create" when old_string is empty (new file), "Update" otherwise.
// Used by the TUI to show contextual tool names matching claude-code-main behavior.
func DynamicUserFacingName(input json.RawMessage) string {
	var in Input
	if json.Unmarshal(input, &in) == nil && in.OldString == "" {
		return "Create"
	}
	return "Update"
}

// DynamicUserFacingNameFromInput returns the dynamic name based on parsed input.
func DynamicUserFacingNameFromInput(input Input) string {
	if input.OldString == "" {
		return "Create"
	}
	return "Update"
}

func (t *FileEditTool) GetActivityDescription(input json.RawMessage) string {
	if p := t.GetPath(input); p != "" {
		return "Editing " + p
	}
	return "Editing file"
}
func (t *FileEditTool) GetToolUseSummary(input json.RawMessage) string {
	return t.GetActivityDescription(input)
}
func (t *FileEditTool) ToAutoClassifierInput(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return ""
	}
	return in.FilePath + " " + in.OldString
}
func (t *FileEditTool) InputsEquivalent(a, b json.RawMessage) bool {
	var ia, ib Input
	if json.Unmarshal(a, &ia) != nil || json.Unmarshal(b, &ib) != nil {
		return false
	}
	return ia.FilePath == ib.FilePath && ia.OldString == ib.OldString && ia.NewString == ib.NewString && ia.ReplaceAll == ib.ReplaceAll
}

func (t *FileEditTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"file_path":{"type":"string","description":"Absolute path to the file to edit."},
			"old_string":{"type":"string","description":"The exact text to replace (must be unique in the file unless replace_all is true)."},
			"new_string":{"type":"string","description":"The replacement text."},
			"replace_all":{"type":"boolean","description":"If true, replace ALL occurrences of old_string. Default false."}
		},
		"required":["file_path","old_string","new_string"]
	}`)
}

func (t *FileEditTool) OutputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"filePath": {"type": "string", "description": "The file path that was edited."},
			"oldString": {"type": "string", "description": "The original string that was replaced."},
			"newString": {"type": "string", "description": "The new string that replaced it."},
			"originalFile": {"type": "string", "description": "The original file contents before editing."},
			"structuredPatch": {"type": "array", "items": {"type": "object", "properties": {"oldStart":{"type":"integer"}, "oldLines":{"type":"integer"}, "newStart":{"type":"integer"}, "newLines":{"type":"integer"}, "lines":{"type":"array","items":{"type":"string"}}}}, "description": "Diff patch showing the changes."},
			"replaceAll": {"type": "boolean", "description": "Whether all occurrences were replaced."}
		}
	}`)
}

func (t *FileEditTool) Prompt(_ *tool.UseContext) string {
	return `Performs exact string replacements in files.

Usage:
- You must use your Read tool at least once in the conversation before editing. This tool will error if you attempt an edit without reading the file.
- When editing text from Read tool output, ensure you preserve the exact indentation (tabs/spaces) as it appears AFTER the line number prefix. The line number prefix format is: spaces + line number + tab. Everything after that tab is the actual file content to match. Never include any part of the line number prefix in the old_string or new_string.
- ALWAYS prefer editing existing files in the codebase. NEVER write new files unless explicitly required.
- Only use emojis if the user explicitly requests it. Avoid adding emojis to files unless asked.
- The edit will FAIL if old_string is not unique in the file. Either provide a larger string with more surrounding context to make it unique or use replace_all to change every instance of old_string.
- Use replace_all for replacing and renaming strings across the file. This parameter is useful if you want to rename a variable for instance.
- The edit will FAIL if old_string and new_string are identical. This is considered a no-op and will throw an error.
- Include an explanation field to describe the change you are making.
IMPORTANT: You must generate the following arguments first, before any others: [file_path]`
}

func (t *FileEditTool) ValidateInput(_ context.Context, input json.RawMessage) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.FilePath == "" {
		return fmt.Errorf("file_path must not be empty")
	}
	if in.OldString == in.NewString {
		return fmt.Errorf("old_string and new_string are identical; this is a no-op")
	}
	if util.IsUNCPath(in.FilePath) {
		return fmt.Errorf("UNC paths are not allowed")
	}
	if util.IsBlockedDevicePath(in.FilePath) {
		return fmt.Errorf("cannot edit device file %q", in.FilePath)
	}
	return nil
}

func (t *FileEditTool) CheckPermissions(_ context.Context, input json.RawMessage, uctx *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.FilePath == "" {
		return fmt.Errorf("file_path must not be empty")
	}
	return nil
}

func (t *FileEditTool) Call(_ context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)

		path := in.FilePath
		if !filepath.IsAbs(path) {
			path = filepath.Join(uctx.WorkDir, path)
		}

		// File size guard.
		statBefore, err := os.Stat(path)
		if err != nil {
			ch <- errBlock("stat file: " + err.Error())
			return
		}
		if statBefore.IsDir() {
			ch <- errBlock(fmt.Sprintf("%q is a directory, not a file", path))
			return
		}
		if statBefore.Size() > util.MaxEditFileSize {
			ch <- errBlock(fmt.Sprintf("file size %d exceeds maximum editable size (%d bytes)", statBefore.Size(), util.MaxEditFileSize))
			return
		}
		modBefore := statBefore.ModTime()

		content, err := os.ReadFile(path)
		if err != nil {
			ch <- errBlock("read file: " + err.Error())
			return
		}
		text := string(content)

		// Detect concurrent modification.
		time.Sleep(1 * time.Millisecond)
		statAfter, err := os.Stat(path)
		if err == nil && statAfter.ModTime() != modBefore {
			ch <- errBlock("file was modified concurrently; please re-read before editing")
			return
		}

		// Find occurrences.
		count := strings.Count(text, in.OldString)
		var newText string
		var replacements int

		if count == 0 {
			// Fuzzy match: try whitespace-normalized search.
			actual := findActualString(text, in.OldString)
			if actual == "" {
				ch <- errBlock(fmt.Sprintf("old_string not found in file %q. Make sure it matches exactly, including whitespace.", path))
				return
			}
			newText = strings.Replace(text, actual, in.NewString, 1)
			replacements = 1
		} else if in.ReplaceAll {
			newText = strings.ReplaceAll(text, in.OldString, in.NewString)
			replacements = count
		} else {
			if count > 1 {
				ch <- errBlock(fmt.Sprintf("old_string appears %d times in file; must be unique (or use replace_all)", count))
				return
			}
			newText = strings.Replace(text, in.OldString, in.NewString, 1)
			replacements = 1
		}

		if err := util.WriteTextContent(path, newText); err != nil {
			ch <- errBlock("write file: " + err.Error())
			return
		}

		// Invalidate read cache so subsequent reads pick up the new content.
		fileread.InvalidateCache(path)

		// Track file history.
		if uctx.UpdateFileHistoryState != nil {
			hash := sha256.Sum256([]byte(newText))
			uctx.UpdateFileHistoryState(func(prev *engine.FileHistoryState) *engine.FileHistoryState {
				if prev == nil {
					prev = &engine.FileHistoryState{Files: make(map[string][]engine.FileSnapshot)}
				}
				if prev.Files == nil {
					prev.Files = make(map[string][]engine.FileSnapshot)
				}
				prev.Files[path] = append(prev.Files[path], engine.FileSnapshot{
					Timestamp: time.Now().UnixMilli(),
					Hash:      hex.EncodeToString(hash[:]),
					ToolUseID: uctx.ToolUseID,
					ToolName:  "Edit",
				})
				return prev
			})
		}

		// Compute diff for result.
		d := diff.Compute(text, newText, path)
		hunks := computeHunks(text, newText)

		var result string
		if in.ReplaceAll && replacements > 1 {
			result = fmt.Sprintf("Successfully replaced %d occurrences in %s\n%s", replacements, path, d.Format())
		} else {
			result = fmt.Sprintf("Successfully edited %s\n%s", path, d.Format())
		}

		// Build structured output (available for downstream consumers).
		_ = Output{
			FilePath:        in.FilePath,
			OldString:       in.OldString,
			NewString:       in.NewString,
			OriginalFile:    text,
			StructuredPatch: hunks,
			ReplaceAll:      in.ReplaceAll,
		}

		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: result,
		}
	}()
	return ch, nil
}

// findActualString attempts a whitespace-normalized fuzzy match.
// If old_string differs from file content only in whitespace, returns the actual
// substring from the file that matches. Returns "" if no match found.
func findActualString(text, search string) string {
	normSearch := normalizeWhitespace(search)
	if normSearch == "" {
		return ""
	}
	// Sliding window: try each position in the file.
	lines := strings.Split(text, "\n")
	searchLines := strings.Split(search, "\n")
	if len(searchLines) == 0 {
		return ""
	}
	for i := 0; i <= len(lines)-len(searchLines); i++ {
		candidate := strings.Join(lines[i:i+len(searchLines)], "\n")
		if normalizeWhitespace(candidate) == normSearch {
			return candidate
		}
	}
	return ""
}

// normalizeWhitespace collapses all runs of whitespace to single spaces and trims.
func normalizeWhitespace(s string) string {
	var sb strings.Builder
	lastWasSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !lastWasSpace {
				sb.WriteRune(' ')
				lastWasSpace = true
			}
		} else {
			sb.WriteRune(r)
			lastWasSpace = false
		}
	}
	return strings.TrimSpace(sb.String())
}

// computeHunks produces StructuredPatchHunk slices from old/new content,
// matching claude-code-main's structuredPatch format for the UI.
func computeHunks(oldContent, newContent string) []StructuredPatchHunk {
	d := diff.Compute(oldContent, newContent, "")
	hunks := make([]StructuredPatchHunk, 0, len(d.Hunks))
	for _, h := range d.Hunks {
		lines := make([]string, 0, len(h.Lines))
		for _, l := range h.Lines {
			switch l.Op {
			case diff.OpEqual:
				lines = append(lines, " "+l.Content)
			case diff.OpInsert:
				lines = append(lines, "+"+l.Content)
			case diff.OpDelete:
				lines = append(lines, "-"+l.Content)
			}
		}
		hunks = append(hunks, StructuredPatchHunk{
			OldStart: h.OldStart,
			OldLines: h.OldCount,
			NewStart: h.NewStart,
			NewLines: h.NewCount,
			Lines:    lines,
		})
	}
	return hunks
}

// MapToolResultToBlockParam formats the edit result for the model.
func (t *FileEditTool) MapToolResultToBlockParam(content interface{}, toolUseID string) *engine.ContentBlock {
	text, ok := content.(string)
	if !ok {
		return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: ""}
	}
	return &engine.ContentBlock{Type: engine.ContentTypeToolResult, ToolUseID: toolUseID, Text: text}
}

func errBlock(msg string) *engine.ContentBlock {
	return &engine.ContentBlock{Type: engine.ContentTypeText, Text: msg, IsError: true}
}
