package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ────────────────────────────────────────────────────────────────────────────
// ToolResultBudget — manages large tool results by persisting them to disk
// and replacing inline content with references.
// Aligned with claude-code-main's toolResultBudget / disk persistence pattern.
// ────────────────────────────────────────────────────────────────────────────

// ToolResultBudgetConfig configures the tool result budget manager.
type ToolResultBudgetConfig struct {
	// MaxInlineChars is the maximum number of characters kept inline.
	// Results exceeding this are persisted to disk and replaced with a reference.
	MaxInlineChars int

	// StorageDir is the directory where large results are persisted.
	// Defaults to a temp directory under the session's scratch space.
	StorageDir string

	// MaxTotalStorageBytes caps the total disk usage for stored results.
	// 0 means unlimited.
	MaxTotalStorageBytes int64
}

// DefaultToolResultBudgetConfig returns sensible defaults.
func DefaultToolResultBudgetConfig() ToolResultBudgetConfig {
	return ToolResultBudgetConfig{
		MaxInlineChars:       100_000, // 100K chars inline
		MaxTotalStorageBytes: 50 * 1024 * 1024, // 50MB total
	}
}

// ToolResultBudget manages large tool results by persisting oversized output
// to disk and replacing it with compact references.
type ToolResultBudget struct {
	mu     sync.Mutex
	config ToolResultBudgetConfig
	// stored tracks all persisted results by hash.
	stored map[string]storedResult
	// totalBytes tracks cumulative disk usage.
	totalBytes int64
}

type storedResult struct {
	Hash     string
	Path     string
	Size     int64
	ToolName string
	ToolID   string
}

// NewToolResultBudget creates a new budget manager.
func NewToolResultBudget(cfg ToolResultBudgetConfig) *ToolResultBudget {
	if cfg.MaxInlineChars <= 0 {
		cfg.MaxInlineChars = DefaultToolResultBudgetConfig().MaxInlineChars
	}
	return &ToolResultBudget{
		config: cfg,
		stored: make(map[string]storedResult),
	}
}

// ProcessResult checks if a tool result exceeds the inline limit.
// If so, it persists the full result to disk and returns a truncated
// version with a reference. Otherwise returns the original blocks unchanged.
func (trb *ToolResultBudget) ProcessResult(toolName, toolID string, blocks []*ContentBlock) []*ContentBlock {
	totalChars := 0
	for _, b := range blocks {
		totalChars += len(b.Text)
	}

	if totalChars <= trb.config.MaxInlineChars {
		return blocks // fits inline
	}

	// Persist to disk.
	fullText := blocksToString(blocks)
	hash := hashContent(fullText)

	trb.mu.Lock()
	defer trb.mu.Unlock()

	// Check if already stored.
	if sr, ok := trb.stored[hash]; ok {
		return trb.makeReference(sr, totalChars)
	}

	// Check storage budget.
	newSize := int64(len(fullText))
	if trb.config.MaxTotalStorageBytes > 0 && trb.totalBytes+newSize > trb.config.MaxTotalStorageBytes {
		// Over budget — just truncate inline without persisting.
		return capBlockSize(blocks, trb.config.MaxInlineChars)
	}

	// Persist to disk.
	path, err := trb.persistToDisk(hash, fullText)
	if err != nil {
		// Fallback: truncate inline.
		return capBlockSize(blocks, trb.config.MaxInlineChars)
	}

	sr := storedResult{
		Hash:     hash,
		Path:     path,
		Size:     newSize,
		ToolName: toolName,
		ToolID:   toolID,
	}
	trb.stored[hash] = sr
	trb.totalBytes += newSize

	return trb.makeReference(sr, totalChars)
}

// RetrieveResult loads a previously persisted result from disk by hash.
func (trb *ToolResultBudget) RetrieveResult(hash string) (string, error) {
	trb.mu.Lock()
	sr, ok := trb.stored[hash]
	trb.mu.Unlock()

	if !ok {
		return "", fmt.Errorf("tool result %q not found in budget store", hash)
	}

	data, err := os.ReadFile(sr.Path)
	if err != nil {
		return "", fmt.Errorf("read stored result: %w", err)
	}
	return string(data), nil
}

// Cleanup removes all persisted results from disk.
func (trb *ToolResultBudget) Cleanup() error {
	trb.mu.Lock()
	defer trb.mu.Unlock()

	var firstErr error
	for _, sr := range trb.stored {
		if err := os.Remove(sr.Path); err != nil && !os.IsNotExist(err) {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	trb.stored = make(map[string]storedResult)
	trb.totalBytes = 0
	return firstErr
}

// Stats returns current budget usage statistics.
func (trb *ToolResultBudget) Stats() ToolResultBudgetStats {
	trb.mu.Lock()
	defer trb.mu.Unlock()
	return ToolResultBudgetStats{
		StoredCount: len(trb.stored),
		TotalBytes:  trb.totalBytes,
		MaxBytes:    trb.config.MaxTotalStorageBytes,
	}
}

// ToolResultBudgetStats reports current storage usage.
type ToolResultBudgetStats struct {
	StoredCount int   `json:"stored_count"`
	TotalBytes  int64 `json:"total_bytes"`
	MaxBytes    int64 `json:"max_bytes"`
}

// ── Internal helpers ────────────────────────────────────────────────────────

func (trb *ToolResultBudget) persistToDisk(hash, content string) (string, error) {
	dir := trb.config.StorageDir
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "agent-engine-results")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	path := filepath.Join(dir, hash+".txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func (trb *ToolResultBudget) makeReference(sr storedResult, originalChars int) []*ContentBlock {
	// Keep first portion inline + append reference.
	preview := ""
	if data, err := os.ReadFile(sr.Path); err == nil {
		preview = string(data)
		maxPreview := trb.config.MaxInlineChars / 2
		if len(preview) > maxPreview {
			preview = preview[:maxPreview]
		}
	}

	ref := fmt.Sprintf(
		"%s\n\n[... %d additional characters stored on disk (hash: %s) ...]",
		preview, originalChars-len(preview), sr.Hash,
	)
	return []*ContentBlock{{
		Type: ContentTypeText,
		Text: ref,
	}}
}

func hashContent(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8]) // 16 hex chars is sufficient for dedup
}
