package buddy

import (
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const buddyFileName = "companion.json"

// StorageDir returns the default directory for companion persistence.
func StorageDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".agent-engine")
}

// SaveStoredCompanion persists only the StoredCompanion (soul + hatchedAt) to disk.
// Bones are recomputed from hash(userId + SALT) on every read.
func SaveStoredCompanion(sc *StoredCompanion, dir string) error {
	if dir == "" {
		dir = StorageDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("buddy save: mkdir: %w", err)
	}
	b, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return fmt.Errorf("buddy save: marshal: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, buddyFileName), b, 0o644)
}

// LoadStoredCompanion reads the StoredCompanion from disk.
// Returns nil, nil if no companion has been hatched yet.
func LoadStoredCompanion(dir string) (*StoredCompanion, error) {
	if dir == "" {
		dir = StorageDir()
	}
	path := filepath.Join(dir, buddyFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no saved companion yet
		}
		return nil, fmt.Errorf("buddy load: %w", err)
	}
	var sc StoredCompanion
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("buddy load: unmarshal: %w", err)
	}
	return &sc, nil
}

// LoadCompanion loads stored companion and merges with deterministic bones.
// Returns nil if no companion hatched yet.
func LoadCompanion(userID, dir string) *Companion {
	sc, _ := LoadStoredCompanion(dir)
	return GetCompanion(userID, sc)
}

// SaveCompanion is a convenience wrapper that saves a Companion's soul + hatchedAt.
func SaveCompanion(c *Companion, dir string) error {
	sc := &StoredCompanion{
		CompanionSoul: c.CompanionSoul,
		HatchedAt:     c.HatchedAt,
	}
	return SaveStoredCompanion(sc, dir)
}

// CompanionMutedFile returns the path to the muted flag file.
func CompanionMutedFile(dir string) string {
	if dir == "" {
		dir = StorageDir()
	}
	return filepath.Join(dir, "companion_muted")
}

// SetCompanionMuted sets or clears the muted state.
func SetCompanionMuted(muted bool, dir string) error {
	path := CompanionMutedFile(dir)
	if muted {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("1"), 0o644)
	}
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// IsCompanionMuted checks if the companion is muted.
func IsCompanionMuted(dir string) bool {
	_, err := os.Stat(CompanionMutedFile(dir))
	return err == nil
}

// ─── Stable User ID ─────────────────────────────────────────────────────────
// Persistent user ID so the companion bones stay the same across sessions.
// claude-code-main uses oauthAccount.accountUuid ?? userID ?? 'anon'.

const userIDFileName = "user_id"

// GetOrCreateUserID returns a stable user identifier persisted in configDir.
// If no ID exists yet, one is generated from a UUID-like random string and saved.
func GetOrCreateUserID(dir string) string {
	if dir == "" {
		dir = StorageDir()
	}
	path := filepath.Join(dir, userIDFileName)
	data, err := os.ReadFile(path)
	if err == nil && len(data) > 0 {
		return string(data)
	}
	// Generate a new stable ID
	id := generateUserID()
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(path, []byte(id), 0o644)
	return id
}

// generateUserID creates a random hex string (16 bytes = 32 hex chars).
func generateUserID() string {
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		// Fallback: use timestamp + process ID
		return fmt.Sprintf("user-%d-%d", os.Getpid(), time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}
