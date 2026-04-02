package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MemoryType classifies a memory entry.
type MemoryType string

const (
	MemoryTypeUser      MemoryType = "user"
	MemoryTypeFeedback  MemoryType = "feedback"
	MemoryTypeProject   MemoryType = "project"
	MemoryTypeReference MemoryType = "reference"
)

// ValidMemoryTypes is the set of recognized memory types.
var ValidMemoryTypes = []MemoryType{
	MemoryTypeUser,
	MemoryTypeFeedback,
	MemoryTypeProject,
	MemoryTypeReference,
}

// ParseMemoryType returns a valid MemoryType or empty string.
func ParseMemoryType(raw string) MemoryType {
	raw = strings.TrimSpace(strings.ToLower(raw))
	for _, t := range ValidMemoryTypes {
		if string(t) == raw {
			return t
		}
	}
	return ""
}

// MemoryHeader holds metadata parsed from a memory file's frontmatter.
type MemoryHeader struct {
	Filename    string     `json:"filename"`
	FilePath    string     `json:"file_path"`
	ModTimeMs   int64      `json:"mtime_ms"`
	Name        string     `json:"name,omitempty"`
	Description string     `json:"description,omitempty"`
	Type        MemoryType `json:"type,omitempty"`
}

const (
	maxMemoryFiles      = 200
	frontmatterMaxLines = 30
)

// ScanMemoryDir scans a directory for .md memory files, reads their
// frontmatter, and returns headers sorted newest-first (capped at 200).
func ScanMemoryDir(memoryDir string) ([]MemoryHeader, error) {
	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan memory dir: %w", err)
	}

	var headers []MemoryHeader
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if strings.ToUpper(e.Name()) == "MEMORY.MD" {
			continue
		}

		filePath := filepath.Join(memoryDir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}

		header := MemoryHeader{
			Filename:  e.Name(),
			FilePath:  filePath,
			ModTimeMs: info.ModTime().UnixMilli(),
		}

		// Read frontmatter from the first few lines.
		data, err := os.ReadFile(filePath)
		if err != nil {
			headers = append(headers, header)
			continue
		}

		fm := parseFrontmatter(string(data))
		if v, ok := fm["name"]; ok {
			header.Name = v
		}
		if v, ok := fm["description"]; ok {
			header.Description = v
		}
		if v, ok := fm["type"]; ok {
			header.Type = ParseMemoryType(v)
		}

		headers = append(headers, header)
	}

	// Sort newest first.
	sort.Slice(headers, func(i, j int) bool {
		return headers[i].ModTimeMs > headers[j].ModTimeMs
	})

	if len(headers) > maxMemoryFiles {
		headers = headers[:maxMemoryFiles]
	}
	return headers, nil
}

// FormatMemoryManifest formats headers as a text listing.
func FormatMemoryManifest(headers []MemoryHeader) string {
	if len(headers) == 0 {
		return "(no memories)"
	}
	var sb strings.Builder
	for _, h := range headers {
		ts := time.UnixMilli(h.ModTimeMs).UTC().Format(time.RFC3339)
		if h.Type != "" {
			fmt.Fprintf(&sb, "- [%s] %s (%s)", h.Type, h.Filename, ts)
		} else {
			fmt.Fprintf(&sb, "- %s (%s)", h.Filename, ts)
		}
		if h.Description != "" {
			sb.WriteString(": ")
			sb.WriteString(h.Description)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// ReadMemoryFile reads a single memory file and returns its content.
func ReadMemoryFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteMemoryFile writes content to a memory file, creating parent dirs.
func WriteMemoryFile(filePath, content string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.WriteFile(filePath, []byte(content), 0600)
}

// DeleteMemoryFile removes a memory file.
func DeleteMemoryFile(filePath string) error {
	err := os.Remove(filePath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// BuildMemoryContent creates a markdown memory file with frontmatter.
func BuildMemoryContent(name, description, body string, memType MemoryType) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", name))
	if description != "" {
		sb.WriteString(fmt.Sprintf("description: %s\n", description))
	}
	if memType != "" {
		sb.WriteString(fmt.Sprintf("type: %s\n", string(memType)))
	}
	sb.WriteString("---\n\n")
	sb.WriteString(body)
	sb.WriteString("\n")
	return sb.String()
}

// parseFrontmatter extracts YAML frontmatter key-value pairs from text.
// Only parses the block between the first pair of "---" delimiters.
func parseFrontmatter(text string) map[string]string {
	lines := strings.SplitN(text, "\n", frontmatterMaxLines+1)
	result := make(map[string]string)

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return result
	}

	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			break
		}
		if idx := strings.IndexByte(line, ':'); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			if key != "" {
				result[key] = value
			}
		}
	}
	return result
}

// GetMemoryDir returns the default memory directory for a project.
func GetMemoryDir(projectDir string) string {
	return filepath.Join(projectDir, ".claude", "memory")
}

// GetUserMemoryDir returns the global user memory directory.
func GetUserMemoryDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "memory")
}
