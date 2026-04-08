package askquestion

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
)

// ──────────────────────────────────────────────────────────────────────────────
// Image Paste Support — Go port of image paste handling in
// AskUserQuestionPermissionRequest.tsx
//
// In claude-code-main, the user can paste images while in the "Other" text
// input or Notes field. The pasted image is stored as a PastedContent and
// displayed as "(Image attached)" in the answer text.
// ──────────────────────────────────────────────────────────────────────────────

// ImagePasteStore manages pasted images per question.
type ImagePasteStore struct {
	mu          sync.Mutex
	perQuestion map[string][]PastedContent
}

// NewImagePasteStore creates an empty store.
func NewImagePasteStore() *ImagePasteStore {
	return &ImagePasteStore{
		perQuestion: make(map[string][]PastedContent),
	}
}

// Add adds a pasted image for a question.
func (s *ImagePasteStore) Add(questionText string, content PastedContent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.perQuestion[questionText] = append(s.perQuestion[questionText], content)
}

// Get returns the pasted images for a question.
func (s *ImagePasteStore) Get(questionText string) []PastedContent {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]PastedContent, len(s.perQuestion[questionText]))
	copy(cp, s.perQuestion[questionText])
	return cp
}

// All returns all pasted images across all questions.
func (s *ImagePasteStore) All() []PastedContent {
	s.mu.Lock()
	defer s.mu.Unlock()
	var all []PastedContent
	for _, items := range s.perQuestion {
		all = append(all, items...)
	}
	return all
}

// Remove removes a pasted image by ID from a question.
func (s *ImagePasteStore) Remove(questionText, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.perQuestion[questionText]
	filtered := make([]PastedContent, 0, len(items))
	for _, item := range items {
		if item.ID != id {
			filtered = append(filtered, item)
		}
	}
	s.perQuestion[questionText] = filtered
}

// HasImages returns true if any question has pasted images.
func (s *ImagePasteStore) HasImages() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, items := range s.perQuestion {
		if len(items) > 0 {
			return true
		}
	}
	return false
}

// ── Detection helpers ───────────────────────────────────────────────────────

// DetectBase64Image checks if a string looks like a base64-encoded image
// and returns a PastedContent if so. Returns nil if not an image.
func DetectBase64Image(data string) *PastedContent {
	data = strings.TrimSpace(data)

	// Check for data URI scheme
	if strings.HasPrefix(data, "data:image/") {
		return parseDataURI(data)
	}

	// Check if it looks like raw base64 (at least 500 chars to avoid false
	// positives on normal text that happens to be valid base64).
	if len(data) >= 500 {
		// Try to decode a small sample to verify it's valid base64
		sample := data
		if len(sample) > 200 {
			sample = sample[:200]
		}
		// Pad sample for valid base64
		for len(sample)%4 != 0 {
			sample += "="
		}
		if _, err := base64.StdEncoding.DecodeString(sample); err == nil {
			return &PastedContent{
				ID:        generateID(),
				Type:      "image",
				Content:   data,
				MediaType: "image/png", // assume PNG for raw base64
			}
		}
	}

	return nil
}

// parseDataURI parses a data:image/... URI into a PastedContent.
func parseDataURI(uri string) *PastedContent {
	// Format: data:image/png;base64,iVBOR...
	commaIdx := strings.Index(uri, ",")
	if commaIdx < 0 {
		return nil
	}

	header := uri[:commaIdx]
	content := uri[commaIdx+1:]

	// Extract media type
	mediaType := "image/png"
	if strings.HasPrefix(header, "data:") {
		meta := header[5:]
		semiIdx := strings.Index(meta, ";")
		if semiIdx > 0 {
			mediaType = meta[:semiIdx]
		} else {
			mediaType = meta
		}
	}

	return &PastedContent{
		ID:        generateID(),
		Type:      "image",
		Content:   content,
		MediaType: mediaType,
	}
}

// generateID creates a short random hex ID.
func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("img_%d", len(b))
	}
	return "img_" + hex.EncodeToString(b)
}

// RenderImageAttachments renders a summary of attached images for display.
func RenderImageAttachments(images []PastedContent) string {
	if len(images) == 0 {
		return ""
	}
	if len(images) == 1 {
		return "(1 image attached)"
	}
	return fmt.Sprintf("(%d images attached)", len(images))
}
