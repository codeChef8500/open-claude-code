package teammem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// ────────────────────────────────────────────────────────────────────────────
// Team memory sync core logic — aligned with claude-code-main
// src/services/teamMemorySync/index.ts (syncTeamMemory)
// ────────────────────────────────────────────────────────────────────────────

// SyncService orchestrates team memory synchronization.
type SyncService struct {
	client *TeamMemClient
}

// NewSyncService creates a new sync service with the given client.
func NewSyncService(client *TeamMemClient) *SyncService {
	return &SyncService{client: client}
}

// Sync performs a full bidirectional sync of team memory.
// 1. Pull: fetch server state, write new/changed files locally
// 2. Push: detect local changes, upload deltas
// 3. Conflict resolution: retry on 412 up to MaxConflictRetries
func (s *SyncService) Sync(ctx context.Context, state *SyncState) (*SyncResult, error) {
	result := &SyncResult{}

	// Ensure team memory directory exists
	teamDir := strings.TrimRight(state.TeamMemDir, string(filepath.Separator))
	if err := os.MkdirAll(teamDir, 0o700); err != nil {
		return nil, &SyncError{Kind: SyncErrorLocal, Message: "create team dir", Cause: err}
	}

	// ── PULL phase ──────────────────────────────────────────────────────
	fetchResult, err := s.client.FetchTeamMemory(ctx, state)
	if err != nil {
		return nil, fmt.Errorf("pull: %w", err)
	}

	if fetchResult.NotFound {
		slog.Info("team memory: no remote state, starting fresh")
	} else {
		pulled, pullErr := s.applyPull(state, fetchResult)
		if pullErr != nil {
			result.Errors = append(result.Errors, pullErr)
		}
		result.PulledCount = pulled
		state.ETag = fetchResult.ETag
		if fetchResult.Hashes != nil {
			state.ServerChecksums = fetchResult.Hashes
		}
	}

	// ── PUSH phase ──────────────────────────────────────────────────────
	localEntries, err := s.readLocalEntries(teamDir)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result, nil
	}

	// Filter secrets
	cleanEntries, skipped := FilterSecretsFromEntries(localEntries)
	result.SkippedFiles = skipped
	if len(skipped) > 0 {
		slog.Warn("team memory: skipped files with secrets",
			slog.Int("count", len(skipped)))
	}

	// Compute deltas
	deltas := s.computeDeltas(cleanEntries, state.ServerChecksums)
	if len(deltas) == 0 {
		slog.Info("team memory: no local changes to push")
		return result, nil
	}

	// Push with conflict retry
	for attempt := 0; attempt < MaxConflictRetries; attempt++ {
		pushResult, pushErr := s.client.UploadTeamMemory(ctx, state, deltas, state.ETag)
		if pushErr != nil {
			result.Errors = append(result.Errors, pushErr)
			return result, nil
		}

		if pushResult.Success {
			result.PushedCount = len(deltas)
			if pushResult.ETag != "" {
				state.ETag = pushResult.ETag
			}
			slog.Info("team memory: push succeeded",
				slog.Int("entries", len(deltas)),
				slog.Int("attempt", attempt+1))
			return result, nil
		}

		if pushResult.Conflict {
			slog.Info("team memory: conflict, retrying",
				slog.Int("attempt", attempt+1))
			// Re-fetch server state for merge
			freshFetch, refetchErr := s.client.FetchTeamMemory(ctx, state)
			if refetchErr != nil {
				result.Errors = append(result.Errors, refetchErr)
				return result, nil
			}
			if freshFetch.Hashes != nil {
				state.ServerChecksums = freshFetch.Hashes
			}
			state.ETag = freshFetch.ETag

			// Re-apply pull
			pulled, _ := s.applyPull(state, freshFetch)
			result.PulledCount += pulled

			// Re-compute deltas against fresh server state
			localEntries, _ = s.readLocalEntries(teamDir)
			cleanEntries, moreSkipped := FilterSecretsFromEntries(localEntries)
			result.SkippedFiles = append(result.SkippedFiles, moreSkipped...)
			deltas = s.computeDeltas(cleanEntries, state.ServerChecksums)
			if len(deltas) == 0 {
				slog.Info("team memory: deltas resolved after merge")
				return result, nil
			}
			continue
		}
	}

	result.Errors = append(result.Errors, &SyncError{
		Kind:    SyncErrorConflict,
		Message: fmt.Sprintf("conflict not resolved after %d retries", MaxConflictRetries),
	})
	return result, nil
}

// applyPull writes fetched entries to the local team memory directory.
// Returns the count of files written.
func (s *SyncService) applyPull(state *SyncState, fetch *FetchResult) (int, error) {
	teamDir := strings.TrimRight(state.TeamMemDir, string(filepath.Separator))
	written := 0

	for _, entry := range fetch.Entries {
		if len(entry.Content) > MaxFileSize {
			slog.Warn("team memory: skipping oversized entry",
				slog.String("key", entry.Key),
				slog.Int("size", len(entry.Content)))
			continue
		}

		localPath := filepath.Join(teamDir, entry.Key)
		// Safety check: ensure path stays within teamDir
		if !isPathUnder(localPath, teamDir) {
			slog.Warn("team memory: rejecting path escape",
				slog.String("key", entry.Key))
			continue
		}

		// Check if local copy is already identical
		existing, err := os.ReadFile(localPath)
		if err == nil && contentChecksum(string(existing)) == contentChecksum(entry.Content) {
			continue
		}

		dir := filepath.Dir(localPath)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return written, &SyncError{Kind: SyncErrorLocal, Message: "create dir for " + entry.Key, Cause: err}
		}
		if err := os.WriteFile(localPath, []byte(entry.Content), 0o600); err != nil {
			return written, &SyncError{Kind: SyncErrorLocal, Message: "write " + entry.Key, Cause: err}
		}
		written++
	}

	return written, nil
}

// readLocalEntries reads all .md files from the team memory directory.
func (s *SyncService) readLocalEntries(teamDir string) ([]MemoryEntry, error) {
	entries, err := os.ReadDir(teamDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, &SyncError{Kind: SyncErrorLocal, Message: "read team dir", Cause: err}
	}

	var result []MemoryEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if len(result) >= MaxLocalEntries {
			slog.Warn("team memory: local entry cap reached", slog.Int("max", MaxLocalEntries))
			break
		}

		filePath := filepath.Join(teamDir, e.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			slog.Warn("team memory: read error", slog.String("file", e.Name()), slog.Any("err", err))
			continue
		}

		if len(data) > MaxFileSize {
			slog.Warn("team memory: skipping oversized local file",
				slog.String("file", e.Name()), slog.Int("size", len(data)))
			continue
		}

		result = append(result, MemoryEntry{
			Key:      e.Name(),
			Content:  string(data),
			Checksum: contentChecksum(string(data)),
		})
	}

	return result, nil
}

// computeDeltas computes entries that differ from server checksums.
func (s *SyncService) computeDeltas(local []MemoryEntry, serverChecksums map[string]string) []DeltaEntry {
	var deltas []DeltaEntry

	localKeys := make(map[string]bool)
	for _, e := range local {
		localKeys[e.Key] = true
		serverHash, exists := serverChecksums[e.Key]
		if !exists || serverHash != e.Checksum {
			deltas = append(deltas, DeltaEntry{
				Key:     e.Key,
				Content: e.Content,
			})
		}
	}

	// Detect deleted entries
	for key := range serverChecksums {
		if !localKeys[key] {
			deltas = append(deltas, DeltaEntry{
				Key:     key,
				Deleted: true,
			})
		}
	}

	return deltas
}

// ── Helpers ────────────────────────────────────────────────────────────────

func contentChecksum(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

func isPathUnder(candidate, dir string) bool {
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absDir, absCandidate)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}
