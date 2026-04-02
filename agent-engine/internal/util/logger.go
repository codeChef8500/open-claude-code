package util

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// LogFormat selects the log output format.
type LogFormat string

const (
	LogFormatText LogFormat = "text"
	LogFormatJSON LogFormat = "json"
)

// LogConfig configures the logger.
type LogConfig struct {
	Verbose bool
	Format  LogFormat
	// FilePath, if set, directs log output to this file (in addition to stderr).
	FilePath string
}

// InitLogger configures the global slog logger.
// If verbose is true, DEBUG level is enabled; otherwise INFO.
func InitLogger(verbose bool) {
	InitLoggerWithConfig(LogConfig{Verbose: verbose})
}

// InitLoggerWithConfig configures the global slog logger with full options.
func InitLoggerWithConfig(cfg LogConfig) {
	level := slog.LevelInfo
	if cfg.Verbose {
		level = slog.LevelDebug
	}

	var w io.Writer = os.Stderr
	if cfg.FilePath != "" {
		_ = os.MkdirAll(filepath.Dir(cfg.FilePath), 0o755)
		f, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			w = io.MultiWriter(os.Stderr, f)
		}
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.Format == LogFormatJSON {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}
	slog.SetDefault(slog.New(handler))
}

// Logger returns the default slog.Logger.
func Logger() *slog.Logger {
	return slog.Default()
}

// ComponentLogger returns a logger with a "component" attribute.
func ComponentLogger(component string) *slog.Logger {
	return slog.Default().With(slog.String("component", component))
}

// SessionLogger returns a logger scoped to a session ID.
func SessionLogger(sessionID string) *slog.Logger {
	return slog.Default().With(slog.String("session_id", sessionID))
}

// RequestLogger returns a logger scoped to a request/turn.
func RequestLogger(sessionID string, turnNum int) *slog.Logger {
	return slog.Default().With(
		slog.String("session_id", sessionID),
		slog.Int("turn", turnNum),
	)
}

// LogDebug logs at DEBUG level.
func LogDebug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

// LogInfo logs at INFO level.
func LogInfo(msg string, args ...any) {
	slog.Info(msg, args...)
}

// LogWarn logs at WARN level.
func LogWarn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// LogError logs at ERROR level.
func LogError(msg string, args ...any) {
	slog.Error(msg, args...)
}
