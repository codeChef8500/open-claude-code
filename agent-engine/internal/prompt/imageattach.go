package prompt

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// LoadImageBlock loads an image from a file path and returns a ContentBlock
// with base64-encoded data suitable for the Anthropic vision API.
func LoadImageBlock(imagePath string) (*engine.ContentBlock, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("read image %s: %w", imagePath, err)
	}

	mediaType := detectMediaType(data, imagePath)
	encoded := base64.StdEncoding.EncodeToString(data)

	return &engine.ContentBlock{
		Type:      engine.ContentTypeImage,
		MediaType: mediaType,
		Data:      encoded,
	}, nil
}

// ImageBlockFromBase64 creates a ContentBlock from a raw base64 string.
func ImageBlockFromBase64(b64data, mediaType string) *engine.ContentBlock {
	if mediaType == "" {
		mediaType = "image/png"
	}
	return &engine.ContentBlock{
		Type:      engine.ContentTypeImage,
		MediaType: mediaType,
		Data:      b64data,
	}
}

// ImageBlockFromURL creates a ContentBlock referencing a remote image URL.
func ImageBlockFromURL(url, mediaType string) *engine.ContentBlock {
	if mediaType == "" {
		mediaType = "image/jpeg"
	}
	return &engine.ContentBlock{
		Type:      engine.ContentTypeImage,
		MediaType: mediaType,
		URL:       url,
	}
}

func detectMediaType(data []byte, path string) string {
	// Prefer MIME sniffing from content.
	detected := http.DetectContentType(data)
	if strings.HasPrefix(detected, "image/") {
		return detected
	}
	// Fall back to extension.
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	}
	return "image/png"
}
