package buddy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const buddyFileName = "companion.json"

// StorageDir returns the default directory for companion persistence.
func StorageDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// Save persists only the CompanionSoul (mutable state) to disk.
// Bones are recomputed from the seed on load.
func Save(soul CompanionSoul, dir string) error {
	if dir == "" {
		dir = StorageDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("buddy save: mkdir: %w", err)
	}
	b, err := json.MarshalIndent(soul, "", "  ")
	if err != nil {
		return fmt.Errorf("buddy save: marshal: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, buddyFileName), b, 0o644)
}

// Load reads the CompanionSoul from disk and reconstructs the Companion by
// re-deriving Bones from the provided seed.
func Load(seed uint32, dir string) (*Companion, error) {
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
	var soul CompanionSoul
	if err := json.Unmarshal(data, &soul); err != nil {
		return nil, fmt.Errorf("buddy load: unmarshal: %w", err)
	}
	return &Companion{
		Bones: GenerateBones(seed),
		Soul:  soul,
	}, nil
}

// LoadOrHatch loads an existing companion or hatches a new one if none exists.
func LoadOrHatch(seed uint32, dir string) *Companion {
	c, _ := Load(seed, dir)
	if c != nil {
		return c
	}
	return HatchWithSeed(seed)
}

// SaveCompanion is a convenience wrapper that saves the soul of a Companion.
func SaveCompanion(c *Companion, dir string) error {
	return Save(c.Soul, dir)
}
