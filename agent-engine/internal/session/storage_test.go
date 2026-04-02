package session

import (
	"testing"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
)

func TestStorage_AppendAndReadTranscript(t *testing.T) {
	dir := t.TempDir()
	store := NewStorage(dir)
	sid := "test-session-1"

	// Append a message entry.
	msg := &engine.Message{
		Role: engine.RoleUser,
		Content: []*engine.ContentBlock{
			{Type: engine.ContentTypeText, Text: "Hello world"},
		},
	}
	if err := store.AppendMessage(sid, msg); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	// Read back.
	entries, err := store.ReadTranscript(sid)
	if err != nil {
		t.Fatalf("ReadTranscript: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Type != EntryTypeMessage {
		t.Errorf("expected type %q, got %q", EntryTypeMessage, entries[0].Type)
	}
	if entries[0].SessionID != sid {
		t.Errorf("expected session %q, got %q", sid, entries[0].SessionID)
	}
}

func TestStorage_ReadTranscript_Empty(t *testing.T) {
	dir := t.TempDir()
	store := NewStorage(dir)
	entries, err := store.ReadTranscript("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries != nil {
		t.Error("expected nil for missing transcript")
	}
}

func TestStorage_SaveAndLoadMeta(t *testing.T) {
	dir := t.TempDir()
	store := NewStorage(dir)
	now := time.Now().Truncate(time.Second)

	meta := &SessionMetadata{
		ID:        "sess-42",
		WorkDir:   "/tmp/proj",
		CreatedAt: now,
		UpdatedAt: now,
		TurnCount: 5,
		Model:     "claude-sonnet-4-20250514",
		Tags:      []string{"test", "ci"},
	}
	if err := store.SaveMeta(meta); err != nil {
		t.Fatalf("SaveMeta: %v", err)
	}

	loaded, err := store.LoadMeta("sess-42")
	if err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if loaded.ID != meta.ID {
		t.Errorf("ID mismatch: %q vs %q", loaded.ID, meta.ID)
	}
	if loaded.TurnCount != 5 {
		t.Errorf("TurnCount: expected 5, got %d", loaded.TurnCount)
	}
	if loaded.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model: expected claude-sonnet-4-20250514, got %q", loaded.Model)
	}
	if len(loaded.Tags) != 2 {
		t.Errorf("Tags: expected 2, got %d", len(loaded.Tags))
	}
}

func TestStorage_ListSessions(t *testing.T) {
	dir := t.TempDir()
	store := NewStorage(dir)
	now := time.Now().Truncate(time.Second)

	for i, id := range []string{"a", "b", "c"} {
		meta := &SessionMetadata{
			ID:        id,
			CreatedAt: now.Add(time.Duration(i) * time.Minute),
			UpdatedAt: now.Add(time.Duration(i) * time.Minute),
		}
		if err := store.SaveMeta(meta); err != nil {
			t.Fatalf("SaveMeta(%s): %v", id, err)
		}
	}

	metas, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(metas) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(metas))
	}
	// Should be sorted newest first.
	if metas[0].ID != "c" {
		t.Errorf("expected newest first (c), got %q", metas[0].ID)
	}
}

func TestStorage_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	store := NewStorage(dir)
	sid := "multi-entry"

	for i := 0; i < 5; i++ {
		entry := &TranscriptEntry{
			Type:      EntryTypeMessage,
			SessionID: sid,
			Timestamp: time.Now(),
			Payload:   map[string]string{"turn": string(rune('0' + i))},
		}
		if err := store.AppendEntry(sid, entry); err != nil {
			t.Fatalf("AppendEntry %d: %v", i, err)
		}
	}

	entries, err := store.ReadTranscript(sid)
	if err != nil {
		t.Fatalf("ReadTranscript: %v", err)
	}
	if len(entries) != 5 {
		t.Errorf("expected 5 entries, got %d", len(entries))
	}
}
