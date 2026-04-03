package fileread

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ledongthuc/pdf"
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
	"github.com/wall-ai/agent-engine/internal/util"
)

const (
	maxChars       = 200_000
	maxLinesToRead = 2000
)

// FileUnchangedStub is returned when the file hasn't changed since last read.
const FileUnchangedStub = "File unchanged since last read. The content from the earlier Read tool_result in this conversation is still current — refer to that instead of re-reading."

type cacheEntry struct {
	content string
	modTime int64 // Unix nano
	size    int64
}

var (
	cacheMu sync.RWMutex
	cache   = make(map[string]cacheEntry)
)

type Input struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Pages    string `json:"pages,omitempty"` // e.g. "1-5" for PDF page ranges
}

type FileReadTool struct{ tool.BaseTool }

func New() *FileReadTool { return &FileReadTool{} }

func (t *FileReadTool) Name() string                             { return "Read" }
func (t *FileReadTool) UserFacingName() string                   { return "read" }
func (t *FileReadTool) Description() string                      { return "Read the contents of a file." }
func (t *FileReadTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (t *FileReadTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (t *FileReadTool) MaxResultSizeChars() int                  { return maxChars }
func (t *FileReadTool) IsEnabled(_ *tool.UseContext) bool        { return true }
func (t *FileReadTool) IsSearchOrRead(_ json.RawMessage) engine.SearchOrReadInfo {
	return engine.SearchOrReadInfo{IsSearch: true}
}
func (t *FileReadTool) GetPath(input json.RawMessage) string {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return ""
	}
	return in.FilePath
}
func (t *FileReadTool) GetActivityDescription(input json.RawMessage) string {
	if p := t.GetPath(input); p != "" {
		return "Reading " + p
	}
	return "Reading file"
}

func (t *FileReadTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"file_path":{"type":"string","description":"Absolute path to the file to read."},
			"offset":{"type":"integer","description":"1-indexed line number to start reading from."},
			"limit":{"type":"integer","description":"Number of lines to read."}
		},
		"required":["file_path"]
	}`)
}

func (t *FileReadTool) OutputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"content": {"type": "string", "description": "File content with line numbers in cat -n format."},
			"file_path": {"type": "string", "description": "Absolute path of the file that was read."},
			"total_lines": {"type": "integer", "description": "Total number of lines in the file."},
			"truncated": {"type": "boolean", "description": "Whether the output was truncated."}
		}
	}`)
}

func (t *FileReadTool) Prompt(_ *tool.UseContext) string {
	return `Reads a file from the local filesystem. You can access any file directly by using this tool.
Assume this tool is able to read all files on the machine. If the User provides a path to a file assume that path is valid. It is okay to read a file that does not exist; an error will be returned.

Usage:
- The file_path parameter must be an absolute path, not a relative path
- By default, it reads up to 2000 lines starting from the beginning of the file
- You can optionally specify a line offset and limit (especially handy for long files), but it's recommended to read the whole file by not providing these parameters
- Results are returned using cat -n format, with line numbers starting at 1
- Any lines longer than 2000 characters will be truncated
- This tool allows reading images (eg PNG, JPG, etc). When reading an image file the contents are presented visually as this is a multimodal LLM.
- This tool can read PDF files (.pdf). For large PDFs (more than 10 pages), you MUST provide the pages parameter to read specific page ranges (e.g., pages: "1-5"). Reading a large PDF without the pages parameter will fail. Maximum 20 pages per request.
- This tool can read Jupyter notebooks (.ipynb files) and returns all cells with their outputs, combining code, text, and visualizations.
- This tool can only read files, not directories. To read a directory, use an ls command via the Bash tool.
- You will regularly be asked to read screenshots. If the user provides a path to a screenshot, ALWAYS use this tool to view the file at the path. This tool will work with all temporary file paths.
- If you read a file that exists but has empty contents you will receive a system reminder warning in place of file contents.`
}

func (t *FileReadTool) ValidateInput(_ context.Context, input json.RawMessage) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.FilePath == "" {
		return fmt.Errorf("file_path must not be empty")
	}
	if util.IsUNCPath(in.FilePath) {
		return fmt.Errorf("UNC paths are not allowed")
	}
	if util.IsBlockedDevicePath(in.FilePath) {
		return fmt.Errorf("cannot read device file %q", in.FilePath)
	}
	return nil
}

func (t *FileReadTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in Input
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.FilePath == "" {
		return fmt.Errorf("file_path must not be empty")
	}
	return nil
}

func (t *FileReadTool) Call(_ context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
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
		info, err := os.Stat(path)
		if err != nil {
			// Suggest similar file on ENOENT.
			if os.IsNotExist(err) {
				msg := fmt.Sprintf("file not found: %s", path)
				if similar := util.FindSimilarFile(path, uctx.WorkDir); similar != "" {
					msg += fmt.Sprintf("\nDid you mean: %s", similar)
				}
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: msg, IsError: true}
				return
			}
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
			return
		}
		if info.IsDir() {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: fmt.Sprintf("%q is a directory, not a file", path), IsError: true}
			return
		}
		if info.Size() > util.MaxReadFileSize {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: fmt.Sprintf("file size %d exceeds maximum readable size (%d bytes); use offset/limit to read a portion", info.Size(), util.MaxReadFileSize), IsError: true}
			return
		}

		// Check if it's an image
		if isImageFile(path) {
			block, err := readImageFile(path)
			if err != nil {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
				return
			}
			ch <- block
			return
		}

		// Check if it's a PDF
		if isPDFFile(path) {
			text, err := readPDFFile(path, in.Pages)
			if err != nil {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
				return
			}
			if len(text) > maxChars {
				text = text[:maxChars] + "\n[... truncated ...]"
			}
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: text}
			return
		}

		// Check if it's a Jupyter notebook
		if isNotebookFile(path) {
			text, err := readNotebookFile(path)
			if err != nil {
				ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
				return
			}
			if len(text) > maxChars {
				text = text[:maxChars] + "\n[... truncated ...]"
			}
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: text}
			return
		}

		// Check unchanged-file optimization: if offset/limit are the same
		// as a previous read and the file hasn't been modified, return a stub.
		cacheMu.RLock()
		prev, cached := cache[path]
		cacheMu.RUnlock()
		if cached && prev.modTime == info.ModTime().UnixNano() && prev.size == info.Size() {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: FileUnchangedStub}
			return
		}

		data, err := os.ReadFile(path)
		if err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
			return
		}

		text := string(data)

		// Empty file warning.
		if len(strings.TrimSpace(text)) == 0 {
			ch <- &engine.ContentBlock{
				Type: engine.ContentTypeText,
				Text: fmt.Sprintf("[System warning: file %s exists but has empty contents]", path),
			}
			// Still cache it.
			cacheMu.Lock()
			cache[path] = cacheEntry{content: text, modTime: info.ModTime().UnixNano(), size: info.Size()}
			cacheMu.Unlock()
			return
		}

		lines := strings.Split(text, "\n")

		// Apply offset/limit
		offset := in.Offset
		limit := in.Limit
		if offset > 0 {
			offset-- // convert to 0-indexed
			if offset >= len(lines) {
				offset = len(lines) - 1
			}
			lines = lines[offset:]
		}
		// Default max lines cap when no explicit limit is given.
		if limit <= 0 {
			limit = maxLinesToRead
		}
		truncatedLines := false
		if limit > 0 && limit < len(lines) {
			lines = lines[:limit]
			truncatedLines = true
		}

		// Number lines
		startLine := in.Offset
		if startLine <= 0 {
			startLine = 1
		}
		var sb strings.Builder
		for i, l := range lines {
			fmt.Fprintf(&sb, "%6d\t%s\n", startLine+i, l)
		}
		if truncatedLines {
			totalLines := len(strings.Split(text, "\n"))
			fmt.Fprintf(&sb, "\n[... %d lines truncated. Use offset/limit to read more. Total lines: %d ...]", totalLines-limit, totalLines)
		}
		result := sb.String()
		if len(result) > maxChars {
			result = result[:maxChars] + "\n[... truncated ...]"
		}

		// Cache for edit validation and unchanged detection.
		cacheMu.Lock()
		cache[path] = cacheEntry{content: text, modTime: info.ModTime().UnixNano(), size: info.Size()}
		cacheMu.Unlock()

		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: result}
	}()
	return ch, nil
}

// GetCached returns a previously-read file content for edit validation.
func GetCached(path string) (string, bool) {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	v, ok := cache[path]
	if !ok {
		return "", false
	}
	return v.content, true
}

// InvalidateCache removes a file from the read cache (e.g. after an edit).
func InvalidateCache(path string) {
	cacheMu.Lock()
	delete(cache, path)
	cacheMu.Unlock()
}

func isImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".bmp", ".ico":
		return true
	}
	return false
}

func readImageFile(path string) (*engine.ContentBlock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(path))
	mediaType := "image/png"
	switch ext {
	case ".jpg", ".jpeg":
		mediaType = "image/jpeg"
	case ".gif":
		mediaType = "image/gif"
	case ".webp":
		mediaType = "image/webp"
	}
	return &engine.ContentBlock{
		Type:      engine.ContentTypeImage,
		MediaType: mediaType,
		Data:      base64.StdEncoding.EncodeToString(data),
	}, nil
}

func isPDFFile(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".pdf"
}

func readPDFFile(path, pages string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("open PDF: %w", err)
	}
	defer f.Close()

	totalPages := r.NumPage()
	if totalPages == 0 {
		return "[Empty PDF document]", nil
	}

	// Parse page range.
	startPage, endPage := 1, totalPages
	if pages != "" {
		_, _ = fmt.Sscanf(pages, "%d-%d", &startPage, &endPage)
		if startPage < 1 {
			startPage = 1
		}
		if endPage > totalPages {
			endPage = totalPages
		}
		if endPage-startPage+1 > 20 {
			endPage = startPage + 19
		}
	} else if totalPages > 10 {
		return "", fmt.Errorf("PDF has %d pages (max 10 without pages parameter). Use pages parameter e.g. pages=\"1-5\"", totalPages)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[PDF: %s — %d total pages, showing pages %d-%d]\n\n", filepath.Base(path), totalPages, startPage, endPage))

	for i := startPage; i <= endPage; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		sb.WriteString(fmt.Sprintf("--- Page %d ---\n", i))
		text, err := p.GetPlainText(nil)
		if err != nil {
			sb.WriteString(fmt.Sprintf("[Error reading page %d: %v]\n", i, err))
			continue
		}
		sb.WriteString(text)
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func isNotebookFile(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".ipynb"
}

// notebookJSON mirrors the minimal .ipynb structure we need.
type notebookJSON struct {
	Cells []notebookCell `json:"cells"`
}
type notebookCell struct {
	CellType string           `json:"cell_type"`
	Source   json.RawMessage  `json:"source"`
	Outputs  []notebookOutput `json:"outputs,omitempty"`
}
type notebookOutput struct {
	OutputType string                     `json:"output_type"`
	Text       json.RawMessage            `json:"text,omitempty"`
	Data       map[string]json.RawMessage `json:"data,omitempty"`
}

func readNotebookFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read notebook: %w", err)
	}

	var nb notebookJSON
	if err := json.Unmarshal(data, &nb); err != nil {
		return "", fmt.Errorf("parse notebook JSON: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[Jupyter Notebook: %s — %d cells]\n\n", filepath.Base(path), len(nb.Cells)))

	for i, cell := range nb.Cells {
		sb.WriteString(fmt.Sprintf("─── Cell %d [%s] ───\n", i, cell.CellType))
		source := decodeNotebookStrOrArray(cell.Source)
		sb.WriteString(source)
		if !strings.HasSuffix(source, "\n") {
			sb.WriteString("\n")
		}

		for _, out := range cell.Outputs {
			switch out.OutputType {
			case "stream", "execute_result", "display_data":
				if out.Text != nil {
					sb.WriteString("[Output]\n")
					sb.WriteString(decodeNotebookStrOrArray(out.Text))
					sb.WriteString("\n")
				}
				if textData, ok := out.Data["text/plain"]; ok {
					sb.WriteString("[Output]\n")
					sb.WriteString(decodeNotebookStrOrArray(textData))
					sb.WriteString("\n")
				}
			case "error":
				sb.WriteString("[Error Output]\n")
			}
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// decodeNotebookStrOrArray handles ipynb source/text which can be a string or []string.
func decodeNotebookStrOrArray(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try as string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Try as array of strings.
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return strings.Join(arr, "")
	}
	return string(raw)
}
