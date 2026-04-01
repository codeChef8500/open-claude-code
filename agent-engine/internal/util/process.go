package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// PIDFileExists reports whether a PID file exists at path.
func PIDFileExists(path string) bool {
	return FileExists(path)
}

// WritePIDFile writes the current process PID to path, creating parent dirs.
func WritePIDFile(path string) error {
	return WriteTextContent(path, strconv.Itoa(os.Getpid())+"\n")
}

// ReadPIDFile reads and parses a PID from path.
func ReadPIDFile(path string) (int, error) {
	content, err := ReadTextFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(content))
	if err != nil {
		return 0, fmt.Errorf("invalid PID file content: %w", err)
	}
	return pid, nil
}

// RemovePIDFile deletes the PID file at path (ignores missing-file errors).
func RemovePIDFile(path string) error {
	err := os.Remove(path)
	if IsENOENT(err) {
		return nil
	}
	return err
}

// IsProcessAlive reports whether the process with the given PID is running.
func IsProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds; Signal(0) checks liveness.
	err = proc.Signal(os.Signal(nil))
	return err == nil
}

// DefaultPIDDir returns the directory used for PID files.
func DefaultPIDDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "pids")
}

// PIDFilePath returns the standard PID file path for a named service.
func PIDFilePath(serviceName string) string {
	return filepath.Join(DefaultPIDDir(), sanitiseKey(serviceName)+".pid")
}
