package session

import (
	"github.com/wall-ai/agent-engine/internal/engine"
)

// Adapter implements engine.SessionWriter backed by Storage.
// It is wired at SDK construction time to avoid an import cycle
// between the engine and session packages.
type Adapter struct {
	storage *Storage
}

// NewAdapter creates a session.Adapter backed by the default storage directory.
func NewAdapter() *Adapter {
	return &Adapter{storage: NewStorage(DefaultStorageDir())}
}

// NewAdapterWithStorage creates a session.Adapter with an explicit storage root.
func NewAdapterWithStorage(rootDir string) *Adapter {
	return &Adapter{storage: NewStorage(rootDir)}
}

// AppendMessage persists a message to the session's JSONL transcript.
func (a *Adapter) AppendMessage(sessionID string, msg *engine.Message) error {
	return a.storage.AppendMessage(sessionID, msg)
}
