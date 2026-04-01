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

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

const maxChars = 200_000

var (
	cacheMu sync.RWMutex
	cache   = make(map[string]string)
)

type Input struct {
	FilePath  string `json:"file_path"`
	Offset    int    `json:"offset,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

type FileReadTool struct{}

func New() *FileReadTool { return &FileReadTool{} }

func (t *FileReadTool) Name() string            { return "Read" }
func (t *FileReadTool) UserFacingName() string  { return "read" }
func (t *FileReadTool) Description() string     { return "Read the contents of a file." }
func (t *FileReadTool) IsReadOnly() bool        { return true }
func (t *FileReadTool) IsConcurrencySafe() bool { return true }
func (t *FileReadTool) MaxResultSizeChars() int { return maxChars }
func (t *FileReadTool) IsEnabled(_ *tool.UseContext) bool { return true }

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

func (t *FileReadTool) Prompt(_ *tool.UseContext) string { return "" }

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

		data, err := os.ReadFile(path)
		if err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
			return
		}

		text := string(data)
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
		if limit > 0 && limit < len(lines) {
			lines = lines[:limit]
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
		result := sb.String()
		if len(result) > maxChars {
			result = result[:maxChars] + "\n[... truncated ...]"
		}

		// Cache for edit validation
		cacheMu.Lock()
		cache[path] = text
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
	return v, ok
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
