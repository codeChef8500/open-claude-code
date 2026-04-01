package test

import (
	"os"
	"path/filepath"
)

// writeTestFile writes content to a file, creating parent dirs as needed.
func writeTestFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
