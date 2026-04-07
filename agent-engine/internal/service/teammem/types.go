package teammem

// ────────────────────────────────────────────────────────────────────────────
// Team memory sync types — aligned with claude-code-main
// src/services/teamMemorySync/types.ts
// ────────────────────────────────────────────────────────────────────────────

const (
	// MaxFileSize is the maximum size of a team memory file in bytes.
	MaxFileSize = 250_000
	// MaxPutBodySize is the maximum size of a PUT request body in bytes.
	MaxPutBodySize = 200_000
	// MaxConflictRetries is the maximum number of conflict retry attempts.
	MaxConflictRetries = 3
	// MaxLocalEntries is the maximum number of local team memory entries.
	MaxLocalEntries = 100
)

// SyncState holds the mutable state for a team memory sync session.
type SyncState struct {
	// OAuthToken is the current OAuth access token.
	OAuthToken string
	// RepoSlug is the repository identifier (owner/repo).
	RepoSlug string
	// ProjectRoot is the project root directory.
	ProjectRoot string
	// ServerChecksums maps keys to server-side content checksums.
	ServerChecksums map[string]string
	// ETag is the last-known ETag from the server.
	ETag string
	// TeamMemDir is the local team memory directory path.
	TeamMemDir string
}

// MemoryEntry represents a single team memory file with its content.
type MemoryEntry struct {
	Key      string `json:"key"`
	Content  string `json:"content"`
	Checksum string `json:"checksum,omitempty"`
}

// FetchResult holds the result of fetching team memory from the server.
type FetchResult struct {
	Entries  []MemoryEntry     `json:"entries"`
	ETag     string            `json:"etag,omitempty"`
	Hashes   map[string]string `json:"hashes,omitempty"`
	NotFound bool              `json:"-"`
}

// PushResult holds the result of pushing team memory to the server.
type PushResult struct {
	Success  bool   `json:"success"`
	Conflict bool   `json:"conflict"`
	ETag     string `json:"etag,omitempty"`
}

// SecretSkippedFile records a file that was skipped due to secret detection.
type SecretSkippedFile struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

// DeltaEntry represents a changed entry to be uploaded.
type DeltaEntry struct {
	Key     string `json:"key"`
	Content string `json:"content,omitempty"`
	Deleted bool   `json:"deleted,omitempty"`
}

// SyncResult holds the overall result of a sync operation.
type SyncResult struct {
	PulledCount  int
	PushedCount  int
	DeletedCount int
	SkippedFiles []SecretSkippedFile
	Errors       []error
}

// SyncError represents a typed sync error.
type SyncError struct {
	Kind    SyncErrorKind
	Message string
	Cause   error
}

func (e *SyncError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *SyncError) Unwrap() error { return e.Cause }

// SyncErrorKind classifies sync errors.
type SyncErrorKind int

const (
	SyncErrorAuth SyncErrorKind = iota
	SyncErrorNetwork
	SyncErrorConflict
	SyncErrorServer
	SyncErrorLocal
	SyncErrorSecret
)
