package session

import (
	"fmt"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// ForkOptions controls how a session fork is created.
type ForkOptions struct {
	// NewSessionID is the ID for the forked session.  Required.
	NewSessionID string
	// FromMessageIndex, if ≥ 0, forks only the first N messages.
	// -1 means fork all messages up to the current state.
	FromMessageIndex int
	// Label is an optional human-readable label stored in the fork metadata.
	Label string
}

// Fork creates a new session that is a copy of srcSessionID up to the
// configured message boundary.  The new session's history starts from the
// same compact boundary as the source (via Resume) if one exists.
//
// The forked session shares no mutable state with the source after creation.
func (s *Storage) Fork(srcSessionID string, opts ForkOptions) error {
	if opts.NewSessionID == "" {
		return fmt.Errorf("fork: NewSessionID must not be empty")
	}

	// Load source messages via Resume to respect compact boundaries.
	messages, summaryText, err := s.Resume(srcSessionID)
	if err != nil {
		return fmt.Errorf("fork: load source %q: %w", srcSessionID, err)
	}

	// Optionally trim to the requested message index.
	if opts.FromMessageIndex >= 0 && opts.FromMessageIndex < len(messages) {
		messages = messages[:opts.FromMessageIndex]
	}

	// Copy source metadata, overriding fork-specific fields.
	srcMeta, _ := s.LoadMeta(srcSessionID)
	now := time.Now()
	meta := &SessionMetadata{
		ID:        opts.NewSessionID,
		CreatedAt: now,
		UpdatedAt: now,
		ForkOf:    srcSessionID,
		ForkLabel: opts.Label,
	}
	if srcMeta != nil {
		meta.WorkDir = srcMeta.WorkDir
		meta.Model = srcMeta.Model
	}
	if err := s.SaveMeta(meta); err != nil {
		return fmt.Errorf("fork: write metadata: %w", err)
	}

	// Write compact summary entry if the source had one.
	if summaryText != "" {
		entry := &TranscriptEntry{
			SessionID: opts.NewSessionID,
			Type:      EntryTypeCompactSummary,
			Payload:   summaryText,
			Timestamp: now,
		}
		if err := s.AppendEntry(opts.NewSessionID, entry); err != nil {
			return fmt.Errorf("fork: write summary entry: %w", err)
		}
	}

	// Write messages.
	for _, msg := range messages {
		entry := &TranscriptEntry{
			SessionID: opts.NewSessionID,
			Type:      EntryTypeMessage,
			Payload:   msg,
			Timestamp: now,
		}
		if err := s.AppendEntry(opts.NewSessionID, entry); err != nil {
			return fmt.Errorf("fork: append message: %w", err)
		}
	}

	return nil
}

// ForkInfo holds minimal information about a session's fork ancestry.
type ForkInfo struct {
	ForkOf    string
	ForkLabel string
}

// GetForkInfo returns the fork ancestry of a session from its metadata.
func (s *Storage) GetForkInfo(sessionID string) (*ForkInfo, error) {
	meta, err := s.LoadMeta(sessionID)
	if err != nil {
		return nil, err
	}
	return &ForkInfo{
		ForkOf:    meta.ForkOf,
		ForkLabel: meta.ForkLabel,
	}, nil
}

// FilterMessages returns only the messages in the slice that match the given role.
func FilterMessages(messages []*engine.Message, role string) []*engine.Message {
	var out []*engine.Message
	for _, m := range messages {
		if string(m.Role) == role {
			out = append(out, m)
		}
	}
	return out
}
