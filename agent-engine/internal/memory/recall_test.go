package memory

import (
	"context"
	"errors"
	"testing"
)

type mockLLMCaller struct {
	response string
	err      error
}

func (m *mockLLMCaller) CompleteSimple(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	return m.response, m.err
}

func TestFindRelevantMemories_LLM(t *testing.T) {
	headers := []MemoryHeader{
		{Filename: "user_prefs.md", Name: "User Preferences", Description: "Go, no ORMs", Type: MemoryTypeUser},
		{Filename: "project_arch.md", Name: "Architecture", Description: "Microservices with gRPC", Type: MemoryTypeProject},
		{Filename: "feedback_tests.md", Name: "Testing", Description: "Always run unit tests", Type: MemoryTypeFeedback},
	}

	caller := &mockLLMCaller{
		response: `{"files": ["project_arch.md", "user_prefs.md"]}`,
	}

	files, err := FindRelevantMemories(context.Background(), caller, "what architecture do we use?", headers, 3)
	if err != nil {
		t.Fatalf("FindRelevantMemories: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	if files[0] != "project_arch.md" {
		t.Errorf("files[0] = %q, want project_arch.md", files[0])
	}
}

func TestFindRelevantMemories_LLMFailure_FallsBack(t *testing.T) {
	headers := []MemoryHeader{
		{Filename: "user_prefs.md", Name: "User Preferences", Description: "Go language no ORMs", Type: MemoryTypeUser},
		{Filename: "project_arch.md", Name: "Architecture", Description: "Microservices gRPC", Type: MemoryTypeProject},
	}

	caller := &mockLLMCaller{err: errors.New("API error")}

	files, err := FindRelevantMemories(context.Background(), caller, "Go language preferences", headers, 3)
	if err != nil {
		t.Fatalf("FindRelevantMemories: %v", err)
	}
	// Should fall back to keyword matching
	if len(files) == 0 {
		t.Error("expected keyword fallback to find matches")
	}
}

func TestFindRelevantMemories_NilCaller(t *testing.T) {
	headers := []MemoryHeader{
		{Filename: "test.md", Name: "Test", Description: "testing things", Type: MemoryTypeFeedback},
	}

	files, err := FindRelevantMemories(context.Background(), nil, "testing", headers, 3)
	if err != nil {
		t.Fatalf("FindRelevantMemories: %v", err)
	}
	if len(files) == 0 {
		t.Error("expected keyword fallback to work with nil caller")
	}
}

func TestFindRelevantMemories_EmptyHeaders(t *testing.T) {
	files, err := FindRelevantMemories(context.Background(), nil, "anything", nil, 3)
	if err != nil {
		t.Fatalf("FindRelevantMemories: %v", err)
	}
	if len(files) != 0 {
		t.Error("expected empty result for empty headers")
	}
}

func TestParseRecallResponse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"valid JSON", `{"files": ["a.md", "b.md"]}`, 2},
		{"JSON with prefix", `Here are the files: {"files": ["a.md"]}`, 1},
		{"empty files", `{"files": []}`, 0},
		{"invalid JSON", `not json`, 0},
		{"no braces", `just text`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRecallResponse(tt.input)
			if len(got) != tt.want {
				t.Errorf("parseRecallResponse(%q) = %d files, want %d", tt.input, len(got), tt.want)
			}
		})
	}
}

func TestKeywordRecall(t *testing.T) {
	headers := []MemoryHeader{
		{Filename: "go_prefs.md", Name: "Go Preferences", Description: "prefers Go over Python", Type: MemoryTypeUser},
		{Filename: "docker.md", Name: "Docker Setup", Description: "docker compose for local dev", Type: MemoryTypeProject},
		{Filename: "testing.md", Name: "Testing Rules", Description: "always run Go tests before commit", Type: MemoryTypeFeedback},
	}

	// Query about Go should match go_prefs and testing (which mentions Go)
	results := keywordRecall("Go testing preferences", headers, 5)
	if len(results) == 0 {
		t.Error("expected keyword matches for 'Go testing preferences'")
	}

	// First result should be the one with most matches
	found := false
	for _, r := range results {
		if r == "go_prefs.md" || r == "testing.md" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected go_prefs.md or testing.md in results, got %v", results)
	}
}

func TestBuildMemoryManifest(t *testing.T) {
	headers := []MemoryHeader{
		{Filename: "test.md", Name: "Test", Description: "desc", Type: MemoryTypeUser, ModTimeMs: 1718400000000},
	}
	manifest := buildMemoryManifest(headers)
	if manifest == "" {
		t.Error("manifest should not be empty")
	}
	if !contains(manifest, "test.md") || !contains(manifest, "user") {
		t.Errorf("manifest missing expected content: %q", manifest)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
