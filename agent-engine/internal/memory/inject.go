package memory

import (
	"context"
	"strings"
	"sync"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/provider"
)

// InjectionResult is the combined memory content ready to embed in a system prompt.
type InjectionResult struct {
	ClaudeMdSection  string // CLAUDE.md merged content
	ExtractedSection string // distilled extracted memories
}

// BuildInjection pre-fetches CLAUDE.md content and optionally extracted memories
// in parallel, returning a combined InjectionResult.
func BuildInjection(
	ctx context.Context,
	prov provider.Provider,
	workDir string,
	messages []*engine.Message,
	sessionID string,
	extractEnabled bool,
) (*InjectionResult, error) {
	var (
		claudeMdInjection *MemoryInjection
		extracted         []*ExtractedMemory
		claudeMdErr       error
		extractErr        error
		wg                sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		claudeMdInjection, claudeMdErr = ReadClaudeMd(workDir)
	}()

	if extractEnabled && prov != nil && len(messages) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			extracted, extractErr = ExtractMemories(ctx, prov, messages, sessionID)
		}()
	}

	wg.Wait()

	if claudeMdErr != nil {
		claudeMdInjection = &MemoryInjection{}
	}
	_ = extractErr // extraction failure is non-fatal

	result := &InjectionResult{}

	if claudeMdInjection != nil && claudeMdInjection.HasContent() {
		result.ClaudeMdSection = claudeMdInjection.MergedContent()
	}

	if len(extracted) > 0 {
		result.ExtractedSection = FormatExtracted(extracted)
	}

	return result, nil
}

// FormatExtracted formats extracted memories as a bullet list for system prompt injection.
func FormatExtracted(memories []*ExtractedMemory) string {
	if len(memories) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Remembered Facts\n\n")
	for _, m := range memories {
		sb.WriteString("- ")
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}
	return sb.String()
}

// InjectIntoSystemPrompt appends memory content to an existing system prompt string.
func InjectIntoSystemPrompt(base string, injection *InjectionResult) string {
	if injection == nil {
		return base
	}
	var parts []string
	if base != "" {
		parts = append(parts, base)
	}
	if injection.ClaudeMdSection != "" {
		parts = append(parts, injection.ClaudeMdSection)
	}
	if injection.ExtractedSection != "" {
		parts = append(parts, injection.ExtractedSection)
	}
	return strings.Join(parts, "\n\n")
}
