package test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/session"
)

func TestSessionStorageAppendRead(t *testing.T) {
	dir := t.TempDir()
	s := session.NewStorage(dir)
	sid := "test-session-001"

	msg := &engine.Message{
		ID:   "msg-1",
		Role: engine.RoleUser,
		Content: []*engine.ContentBlock{
			{Type: engine.ContentTypeText, Text: "Hello from test"},
		},
	}
	require.NoError(t, s.AppendMessage(sid, msg))

	entries, err := s.ReadTranscript(sid)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, session.EntryTypeMessage, entries[0].Type)
}

func TestSessionStorageSaveLoadMeta(t *testing.T) {
	dir := t.TempDir()
	s := session.NewStorage(dir)

	meta := &session.SessionMetadata{
		ID:        "meta-session-001",
		WorkDir:   "/tmp/work",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		TurnCount: 3,
		CostUSD:   0.0042,
	}
	require.NoError(t, s.SaveMeta(meta))

	loaded, err := s.LoadMeta(meta.ID)
	require.NoError(t, err)
	assert.Equal(t, meta.ID, loaded.ID)
	assert.Equal(t, meta.WorkDir, loaded.WorkDir)
	assert.InDelta(t, meta.CostUSD, loaded.CostUSD, 1e-9)
}

func TestSessionStorageExportMarkdown(t *testing.T) {
	dir := t.TempDir()
	s := session.NewStorage(dir)
	sid := "export-session-001"

	require.NoError(t, s.SaveMeta(&session.SessionMetadata{
		ID:        sid,
		WorkDir:   "/tmp",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}))

	msgs := []*engine.Message{
		{
			ID:   "m1",
			Role: engine.RoleUser,
			Content: []*engine.ContentBlock{
				{Type: engine.ContentTypeText, Text: "What is 2+2?"},
			},
		},
		{
			ID:   "m2",
			Role: engine.RoleAssistant,
			Content: []*engine.ContentBlock{
				{Type: engine.ContentTypeText, Text: "It is 4."},
			},
		},
	}
	for _, m := range msgs {
		require.NoError(t, s.AppendMessage(sid, m))
	}

	md, err := s.ExportMarkdown(sid)
	require.NoError(t, err)
	assert.Contains(t, md, "Session Transcript")
	assert.Contains(t, md, "What is 2+2?")
	assert.Contains(t, md, "It is 4.")
}

func TestSessionStorageListSessions(t *testing.T) {
	dir := t.TempDir()
	s := session.NewStorage(dir)

	for _, id := range []string{"s1", "s2", "s3"} {
		require.NoError(t, s.SaveMeta(&session.SessionMetadata{
			ID:        id,
			WorkDir:   "/tmp",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}))
	}

	metas, err := s.ListSessions()
	require.NoError(t, err)
	assert.Len(t, metas, 3)
}

// payloadToMessage is shared with export_test — defined here to avoid duplication.
func init() {
	_ = json.Marshal // ensure json is used
}
