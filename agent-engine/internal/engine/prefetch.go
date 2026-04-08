package engine

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// Attachment & Prefetch system — pre-loads memory, skills, and attachments
// concurrently before the first API call.
// Aligned with claude-code-main QueryEngine.ts prefetch pattern.
// ────────────────────────────────────────────────────────────────────────────

// PrefetchResult holds the results of all prefetch operations.
type PrefetchResult struct {
	// MemoryContent is the loaded memory text.
	MemoryContent string
	// MemoryError is set if memory loading failed.
	MemoryError error
	// Attachments are file/URL attachments resolved from the user message.
	Attachments []*Attachment
	// AttachmentErrors are errors from individual attachment loads.
	AttachmentErrors []error
	// SkillContent is the loaded skill/prompt text.
	SkillContent string
	// Duration is the total time taken for all prefetch operations.
	Duration time.Duration
}

// Attachment represents a file or URL attachment resolved for the query.
// Aligned with claude-code-main attachments system.
type Attachment struct {
	// Type is the attachment type: "file", "url", "image".
	Type string `json:"type"`
	// Path is the file path or URL.
	Path string `json:"path"`
	// Content is the resolved content.
	Content string `json:"content,omitempty"`
	// MediaType is the MIME type for images.
	MediaType string `json:"media_type,omitempty"`
	// Data is base64-encoded image data.
	Data string `json:"data,omitempty"`
	// SizeBytes is the content size.
	SizeBytes int `json:"size_bytes,omitempty"`
	// Error is set if this attachment failed to load.
	Error string `json:"error,omitempty"`
}

// PrefetchConfig controls which prefetch operations run.
type PrefetchConfig struct {
	// LoadMemory enables memory loading.
	LoadMemory bool
	// LoadSkills enables skill/prompt loading.
	LoadSkills bool
	// ResolveAttachments enables attachment resolution.
	ResolveAttachments bool
	// AttachmentPaths are the paths/URLs to resolve.
	AttachmentPaths []string
	// Timeout caps the total prefetch time.
	Timeout time.Duration
}

// DefaultPrefetchConfig returns sensible defaults.
func DefaultPrefetchConfig() PrefetchConfig {
	return PrefetchConfig{
		LoadMemory:         true,
		LoadSkills:         true,
		ResolveAttachments: true,
		Timeout:            10 * time.Second,
	}
}

// RunPrefetch executes all configured prefetch operations concurrently.
// It returns when all operations complete or the timeout is reached.
func RunPrefetch(
	ctx context.Context,
	e *Engine,
	deps *QueryDeps,
	cfg PrefetchConfig,
) *PrefetchResult {
	start := time.Now()
	result := &PrefetchResult{}

	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	var wg sync.WaitGroup

	// ── Memory loading ───────────────────────────────────────────────────
	if cfg.LoadMemory {
		wg.Add(1)
		go func() {
			defer wg.Done()
			loader := e.memoryLoader
			if deps != nil && deps.MemoryLoader != nil {
				loader = deps.MemoryLoader
			}
			if loader == nil {
				return
			}
			content, err := loader.LoadMemory(e.WorkDir())
			if err != nil {
				slog.Warn("prefetch: memory loading failed", slog.Any("err", err))
				result.MemoryError = err
				return
			}
			result.MemoryContent = content
		}()
	}

	// ── Attachment resolution ────────────────────────────────────────────
	if cfg.ResolveAttachments && len(cfg.AttachmentPaths) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			attachments, errors := resolveAttachments(ctx, cfg.AttachmentPaths)
			result.Attachments = attachments
			result.AttachmentErrors = errors
		}()
	}

	wg.Wait()
	result.Duration = time.Since(start)

	slog.Debug("prefetch: complete",
		slog.Duration("duration", result.Duration),
		slog.Int("attachments", len(result.Attachments)))

	return result
}

// resolveAttachments resolves attachment paths to content.
func resolveAttachments(ctx context.Context, paths []string) ([]*Attachment, []error) {
	var (
		mu          sync.Mutex
		attachments []*Attachment
		errors      []error
		wg          sync.WaitGroup
	)

	for _, path := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			att := resolveOneAttachment(ctx, p)
			mu.Lock()
			defer mu.Unlock()
			attachments = append(attachments, att)
			if att.Error != "" {
				errors = append(errors, &AttachmentError{Path: p, Message: att.Error})
			}
		}(path)
	}

	wg.Wait()
	return attachments, errors
}

// resolveOneAttachment resolves a single attachment path.
func resolveOneAttachment(_ context.Context, path string) *Attachment {
	// Placeholder — actual implementation would read files, fetch URLs, etc.
	return &Attachment{
		Type:  "file",
		Path:  path,
		Error: "attachment resolution not yet implemented",
	}
}

// AttachmentError is an error from attachment resolution.
type AttachmentError struct {
	Path    string
	Message string
}

func (e *AttachmentError) Error() string {
	return "attachment " + e.Path + ": " + e.Message
}

// ── Attachment → Message conversion ──────────────────────────────────────

// AttachmentsToContentBlocks converts resolved attachments into content blocks
// for injection into the user message.
func AttachmentsToContentBlocks(attachments []*Attachment) []*ContentBlock {
	var blocks []*ContentBlock
	for _, att := range attachments {
		if att.Error != "" {
			continue
		}
		switch att.Type {
		case "image":
			blocks = append(blocks, &ContentBlock{
				Type:      ContentTypeImage,
				MediaType: att.MediaType,
				Data:      att.Data,
			})
		case "file", "url":
			if att.Content != "" {
				blocks = append(blocks, &ContentBlock{
					Type: ContentTypeText,
					Text: att.Content,
				})
			}
		}
	}
	return blocks
}
