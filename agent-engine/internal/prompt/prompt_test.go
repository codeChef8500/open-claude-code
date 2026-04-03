package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// ── ProcessUserInput tests ──────────────────────────────────────────────────

func TestProcessUserInput_SlashCommand(t *testing.T) {
	pi := ProcessUserInput("/compact --force", "/tmp", nil)
	if pi.Command != "compact" {
		t.Errorf("expected command 'compact', got %q", pi.Command)
	}
	if len(pi.CommandArgs) != 1 || pi.CommandArgs[0] != "--force" {
		t.Errorf("expected args ['--force'], got %v", pi.CommandArgs)
	}
}

func TestProcessUserInput_PlainText(t *testing.T) {
	pi := ProcessUserInput("  hello world  ", "/tmp", nil)
	if pi.Command != "" {
		t.Errorf("expected no command, got %q", pi.Command)
	}
	if pi.Text != "hello world" {
		t.Errorf("expected trimmed text, got %q", pi.Text)
	}
}

func TestProcessUserInput_EmptySlash(t *testing.T) {
	pi := ProcessUserInput("/", "/tmp", nil)
	// "/" with no command name — Fields produces ["/"], so Command = ""
	if pi.Command != "" {
		t.Errorf("expected empty command for bare '/', got %q", pi.Command)
	}
}

func TestProcessUserInput_WithImages(t *testing.T) {
	imgs := []*engine.ContentBlock{
		{Type: engine.ContentTypeImage, Text: "img_data"},
	}
	pi := ProcessUserInput("show me", "/tmp", imgs)
	if len(pi.Images) != 1 {
		t.Errorf("expected 1 image, got %d", len(pi.Images))
	}
}

func TestBuildContentBlocks(t *testing.T) {
	pi := &ProcessedInput{
		Text: "hello",
		Images: []*engine.ContentBlock{
			{Type: engine.ContentTypeImage, Text: "data"},
		},
	}
	blocks := BuildContentBlocks(pi)
	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Type != engine.ContentTypeText {
		t.Error("first block should be text")
	}
}

func TestBuildContentBlocks_EmptyText(t *testing.T) {
	pi := &ProcessedInput{Text: ""}
	blocks := BuildContentBlocks(pi)
	if len(blocks) != 0 {
		t.Errorf("expected 0 blocks for empty input, got %d", len(blocks))
	}
}

// ── CacheSection tests ──────────────────────────────────────────────────────

func TestSplitCacheSections_NoMarker(t *testing.T) {
	sections := SplitCacheSections("hello world")
	if len(sections) != 1 || sections[0] != "hello world" {
		t.Errorf("expected single section, got %v", sections)
	}
}

func TestSplitCacheSections_WithMarker(t *testing.T) {
	text := "stable part\n<!-- cache-break -->\ndynamic part"
	sections := SplitCacheSections(text)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	if !strings.Contains(sections[0], "stable") {
		t.Error("first section should contain stable part")
	}
	if !strings.Contains(sections[1], "dynamic") {
		t.Error("second section should contain dynamic part")
	}
}

func TestBuildPartsFromSections(t *testing.T) {
	sections := []string{"part1", "", "part2"}
	parts := BuildPartsFromSections(sections)
	// Empty section should be skipped.
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if !parts[0].CacheHint {
		t.Error("first part should have cache hint")
	}
	if parts[1].CacheHint {
		t.Error("second part should not have cache hint")
	}
}

func TestAddCacheBreak(t *testing.T) {
	result := AddCacheBreak("prompt text\n")
	if !strings.Contains(result, CacheSectionMarker) {
		t.Error("should contain cache section marker")
	}
}

// ── Include tests ───────────────────────────────────────────────────────────

func TestResolveIncludes_NoDirectives(t *testing.T) {
	content := "just plain text"
	result := ResolveIncludes(content, "/tmp")
	if result != content {
		t.Errorf("expected unchanged, got %q", result)
	}
}

func TestResolveIncludes_FileNotFound(t *testing.T) {
	content := "@include ./nonexistent.md"
	result := ResolveIncludes(content, "/tmp")
	if !strings.Contains(result, "<!-- @include error:") {
		t.Errorf("expected error comment, got %q", result)
	}
}

func TestResolveIncludes_Success(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "inc.md"), []byte("included content"), 0644)

	content := "@include ./inc.md"
	result := ResolveIncludes(content, dir)
	if !strings.Contains(result, "included content") {
		t.Errorf("expected included content, got %q", result)
	}
}

func TestResolveIncludes_DepthLimit(t *testing.T) {
	dir := t.TempDir()
	// Create a self-referencing include to test depth limit.
	os.WriteFile(filepath.Join(dir, "self.md"), []byte("@include ./self.md"), 0644)

	content := "@include ./self.md"
	result := ResolveIncludes(content, dir)
	// Should not infinitely recurse; cycle detection should kick in.
	if strings.Count(result, "@include") > maxIncludeDepth+2 {
		t.Error("include depth limit not respected")
	}
}

func TestLoadClaudeMD_NotFound(t *testing.T) {
	dir := t.TempDir()
	content, err := LoadClaudeMD(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty string for missing CLAUDE.md, got %q", content)
	}
}

func TestLoadClaudeMD_Found(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Project\nHello"), 0644)

	content, err := LoadClaudeMD(dir)
	if err != nil {
		t.Fatalf("LoadClaudeMD: %v", err)
	}
	if !strings.Contains(content, "# Project") {
		t.Errorf("expected CLAUDE.md content, got %q", content)
	}
}

func TestLoadClaudeMD_ParentDir(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "subdir")
	os.MkdirAll(child, 0755)
	os.WriteFile(filepath.Join(parent, "CLAUDE.md"), []byte("parent md"), 0644)

	content, err := LoadClaudeMD(child)
	if err != nil {
		t.Fatalf("LoadClaudeMD: %v", err)
	}
	if !strings.Contains(content, "parent md") {
		t.Errorf("expected parent CLAUDE.md content, got %q", content)
	}
}
