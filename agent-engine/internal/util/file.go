package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WriteTextContent atomically writes content to filePath.
// It writes to a temp file first and then renames, so a partial write never
// corrupts the target file.
func WriteTextContent(filePath, content string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("WriteTextContent mkdir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(filePath)+".tmp.*")
	if err != nil {
		return fmt.Errorf("WriteTextContent create temp: %w", err)
	}
	tmpName := tmp.Name()

	defer func() {
		tmp.Close()
		// Clean up the temp file if rename failed.
		os.Remove(tmpName)
	}()

	if _, err := io.WriteString(tmp, content); err != nil {
		return fmt.Errorf("WriteTextContent write: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("WriteTextContent sync: %w", err)
	}
	tmp.Close()

	if err := os.Rename(tmpName, filePath); err != nil {
		return fmt.Errorf("WriteTextContent rename: %w", err)
	}
	return nil
}

// GetFileModificationTime returns the last modification time of filePath.
func GetFileModificationTime(filePath string) (time.Time, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// FileExists reports whether filePath exists and is a regular file.
func FileExists(filePath string) bool {
	info, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// DirExists reports whether dirPath exists and is a directory.
func DirExists(dirPath string) bool {
	info, err := os.Stat(dirPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// FindSimilarFile searches cwd recursively for a file whose base name matches
// the base name of filePath (case-insensitive). Returns "" if not found.
func FindSimilarFile(filePath, cwd string) string {
	baseName := strings.ToLower(filepath.Base(filePath))
	var found string
	_ = filepath.WalkDir(cwd, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			// Skip hidden directories.
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.ToLower(d.Name()) == baseName {
			found = path
			return io.EOF // abuse EOF as a "stop walking" sentinel
		}
		return nil
	})
	return found
}

// ReadTextFile reads the entire contents of filePath as a string.
func ReadTextFile(filePath string) (string, error) {
	b, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// EnsureDir creates all directories in path if they don't exist.
func EnsureDir(dirPath string) error {
	return os.MkdirAll(dirPath, 0o755)
}
