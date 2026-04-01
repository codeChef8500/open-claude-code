package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/util"
)

// Storage persists session transcripts as JSONL files under a root directory.
type Storage struct {
	rootDir string
}

// NewStorage creates a Storage rooted at rootDir.
func NewStorage(rootDir string) *Storage {
	return &Storage{rootDir: rootDir}
}

// DefaultStorageDir returns ~/.claude/sessions.
func DefaultStorageDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "sessions")
}

// SessionDir returns the directory for a specific session.
func (s *Storage) SessionDir(sessionID string) string {
	return filepath.Join(s.rootDir, sessionID)
}

// TranscriptPath returns the JSONL transcript file path for a session.
func (s *Storage) TranscriptPath(sessionID string) string {
	return filepath.Join(s.SessionDir(sessionID), "transcript.jsonl")
}

// MetaPath returns the metadata JSON file path for a session.
func (s *Storage) MetaPath(sessionID string) string {
	return filepath.Join(s.SessionDir(sessionID), "meta.json")
}

// AppendEntry atomically appends a TranscriptEntry to the session JSONL.
func (s *Storage) AppendEntry(sessionID string, entry *TranscriptEntry) error {
	path := s.TranscriptPath(sessionID)
	if err := util.EnsureDir(s.SessionDir(sessionID)); err != nil {
		return fmt.Errorf("session dir: %w", err)
	}

	b, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open transcript: %w", err)
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "%s\n", b)
	return err
}

// AppendMessage is a convenience helper to append a message entry.
func (s *Storage) AppendMessage(sessionID string, msg *engine.Message) error {
	return s.AppendEntry(sessionID, &TranscriptEntry{
		Type:      EntryTypeMessage,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Payload:   msg,
	})
}

// ReadTranscript reads all transcript entries from a session JSONL file.
func (s *Storage) ReadTranscript(sessionID string) ([]*TranscriptEntry, error) {
	path := s.TranscriptPath(sessionID)
	f, err := os.Open(path)
	if err != nil {
		if util.IsENOENT(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []*TranscriptEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry TranscriptEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, &entry)
	}
	return entries, scanner.Err()
}

// SaveMeta writes session metadata to meta.json.
func (s *Storage) SaveMeta(meta *SessionMetadata) error {
	path := s.MetaPath(meta.ID)
	if err := util.EnsureDir(s.SessionDir(meta.ID)); err != nil {
		return err
	}
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return util.WriteTextContent(path, string(b))
}

// LoadMeta reads session metadata from meta.json.
func (s *Storage) LoadMeta(sessionID string) (*SessionMetadata, error) {
	path := s.MetaPath(sessionID)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta SessionMetadata
	if err := json.Unmarshal(b, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// ListSessions returns metadata for all stored sessions, sorted by UpdatedAt desc.
func (s *Storage) ListSessions() ([]*SessionMetadata, error) {
	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		if util.IsENOENT(err) {
			return nil, nil
		}
		return nil, err
	}

	var metas []*SessionMetadata
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		meta, err := s.LoadMeta(e.Name())
		if err != nil {
			continue
		}
		metas = append(metas, meta)
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].UpdatedAt.After(metas[j].UpdatedAt)
	})
	return metas, nil
}
