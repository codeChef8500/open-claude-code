package memory

import (
	"testing"
	"time"
)

func makeMemory(id, content, sessionID string, age time.Duration) *ExtractedMemory {
	return &ExtractedMemory{
		ID:          id,
		Content:     content,
		SessionID:   sessionID,
		ExtractedAt: time.Now().Add(-age),
		Tags:        []string{"test"},
	}
}

func TestStore_SaveAndGet(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	mem := makeMemory("m1", "remember this", "sess-1", 0)
	if err := store.Save(mem); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got := store.Get("m1")
	if got == nil {
		t.Fatal("expected non-nil memory")
	}
	if got.Content != "remember this" {
		t.Errorf("content mismatch: %q", got.Content)
	}
}

func TestStore_LoadFromDisk(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_ = store.Save(makeMemory("a", "alpha", "s1", 0))
	_ = store.Save(makeMemory("b", "beta", "s1", time.Minute))

	// Create a fresh store and load from disk.
	store2 := NewStore(dir)
	if err := store2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if store2.Count() != 2 {
		t.Errorf("expected 2 memories, got %d", store2.Count())
	}
	if store2.Get("a") == nil || store2.Get("b") == nil {
		t.Error("expected both memories to be loaded")
	}
}

func TestStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_ = store.Save(makeMemory("del-me", "temporary", "s1", 0))
	if store.Count() != 1 {
		t.Fatalf("expected 1, got %d", store.Count())
	}

	if err := store.Delete("del-me"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if store.Count() != 0 {
		t.Errorf("expected 0 after delete, got %d", store.Count())
	}
	if store.Get("del-me") != nil {
		t.Error("expected nil after delete")
	}
}

func TestStore_All_SortedByTime(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_ = store.Save(makeMemory("old", "old content", "s1", 10*time.Minute))
	_ = store.Save(makeMemory("new", "new content", "s1", 0))
	_ = store.Save(makeMemory("mid", "mid content", "s1", 5*time.Minute))

	all := store.All()
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
	// Newest first.
	if all[0].ID != "new" {
		t.Errorf("expected newest first, got %q", all[0].ID)
	}
	if all[2].ID != "old" {
		t.Errorf("expected oldest last, got %q", all[2].ID)
	}
}

func TestStore_BySession(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_ = store.Save(makeMemory("m1", "a", "sess-A", 0))
	_ = store.Save(makeMemory("m2", "b", "sess-B", 0))
	_ = store.Save(makeMemory("m3", "c", "sess-A", time.Minute))

	result := store.BySession("sess-A")
	if len(result) != 2 {
		t.Errorf("expected 2 for sess-A, got %d", len(result))
	}
}

func TestStore_ByType(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	m1 := makeMemory("t1", "tagged", "s1", 0)
	m1.Tags = []string{"preference", "test"}
	_ = store.Save(m1)

	m2 := makeMemory("t2", "other", "s1", 0)
	m2.Tags = []string{"code"}
	_ = store.Save(m2)

	prefs := store.ByType("preference")
	if len(prefs) != 1 {
		t.Errorf("expected 1 preference memory, got %d", len(prefs))
	}
}

func TestStore_Clear(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_ = store.Save(makeMemory("c1", "a", "s1", 0))
	_ = store.Save(makeMemory("c2", "b", "s1", 0))

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if store.Count() != 0 {
		t.Errorf("expected 0 after clear, got %d", store.Count())
	}
}

func TestStore_ClearOlderThan(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_ = store.Save(makeMemory("recent", "new", "s1", time.Minute))
	_ = store.Save(makeMemory("old", "ancient", "s1", 2*time.Hour))

	removed, err := store.ClearOlderThan(time.Hour)
	if err != nil {
		t.Fatalf("ClearOlderThan: %v", err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
	if store.Count() != 1 {
		t.Errorf("expected 1 remaining, got %d", store.Count())
	}
}

func TestStore_Deduplicate(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_ = store.Save(makeMemory("dup1", "same content", "s1", 10*time.Minute))
	_ = store.Save(makeMemory("dup2", "same content", "s1", 0))
	_ = store.Save(makeMemory("unique", "different", "s1", 0))

	removed := store.Deduplicate()
	if removed != 1 {
		t.Errorf("expected 1 dedup removal, got %d", removed)
	}
	if store.Count() != 2 {
		t.Errorf("expected 2 remaining, got %d", store.Count())
	}
	// The newer duplicate should survive.
	if store.Get("dup2") == nil {
		t.Error("expected dup2 (newer) to survive")
	}
}

func TestStore_SaveAll(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	mems := []*ExtractedMemory{
		makeMemory("sa1", "first", "s1", 0),
		makeMemory("sa2", "second", "s1", 0),
	}
	if err := store.SaveAll(mems); err != nil {
		t.Fatalf("SaveAll: %v", err)
	}
	if store.Count() != 2 {
		t.Errorf("expected 2, got %d", store.Count())
	}
}
