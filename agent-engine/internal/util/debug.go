package util

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	verboseMode bool
	verboseMu   sync.RWMutex

	diagFile   *os.File
	diagFileMu sync.Mutex
)

// SetVerboseMode enables or disables verbose debug logging.
func SetVerboseMode(v bool) {
	verboseMu.Lock()
	defer verboseMu.Unlock()
	verboseMode = v
}

// IsVerboseMode reports whether verbose debug logging is enabled.
func IsVerboseMode() bool {
	verboseMu.RLock()
	defer verboseMu.RUnlock()
	return verboseMode
}

// LogForDebugging logs a message to stderr only when verbose mode is active.
// If data is non-nil it is pretty-printed as JSON.
func LogForDebugging(message string, data interface{}) {
	if !IsVerboseMode() {
		return
	}
	ts := time.Now().Format(time.RFC3339)
	fmt.Fprintf(os.Stderr, "[DEBUG %s] %s\n", ts, message)
	if data != nil {
		b, err := json.MarshalIndent(data, "", "  ")
		if err == nil {
			fmt.Fprintf(os.Stderr, "%s\n", b)
		}
	}
}

// LogForDiagnosticsNoPII writes a structured log entry to
// ~/.claude/diagnostics.log (no PII). The file is opened lazily.
func LogForDiagnosticsNoPII(level slog.Level, msg string, attrs ...slog.Attr) {
	diagFileMu.Lock()
	defer diagFileMu.Unlock()

	if diagFile == nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		dir := filepath.Join(home, ".claude")
		_ = os.MkdirAll(dir, 0o700)
		f, err := os.OpenFile(filepath.Join(dir, "diagnostics.log"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return
		}
		diagFile = f
	}

	rec := slog.NewRecord(time.Now(), level, msg, 0)
	for _, a := range attrs {
		rec.AddAttrs(a)
	}
	handler := slog.NewJSONHandler(diagFile, nil)
	_ = handler.Handle(nil, rec) //nolint:staticcheck
}

// CloseDiagnosticsLog flushes and closes the diagnostics log file if open.
func CloseDiagnosticsLog() {
	diagFileMu.Lock()
	defer diagFileMu.Unlock()
	if diagFile != nil {
		_ = diagFile.Close()
		diagFile = nil
	}
}
