package util

import (
	"log/slog"
	"os"
)

// InitLogger configures the global slog logger.
// If verbose is true, DEBUG level is enabled; otherwise INFO.
func InitLogger(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}

// Logger returns the default slog.Logger.
func Logger() *slog.Logger {
	return slog.Default()
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
