package util

import (
	"errors"
	"fmt"
	"os"
)

// AbortError signals that an operation was intentionally cancelled.
// Maps to context.Canceled / context.DeadlineExceeded in most cases but
// can also be raised explicitly (e.g. user hit Ctrl-C).
type AbortError struct {
	msg string
}

func NewAbortError(msg string) *AbortError {
	if msg == "" {
		msg = "操作被中止"
	}
	return &AbortError{msg: msg}
}

func (e *AbortError) Error() string { return e.msg }

// Is makes errors.Is(err, &AbortError{}) work for any *AbortError.
func (e *AbortError) Is(target error) bool {
	_, ok := target.(*AbortError)
	return ok
}

// ShellError is returned when a shell command exits with a non-zero code.
type ShellError struct {
	Message  string
	ExitCode int
	Stderr   string
}

func NewShellError(message string, exitCode int, stderr string) *ShellError {
	return &ShellError{Message: message, ExitCode: exitCode, Stderr: stderr}
}

func (e *ShellError) Error() string {
	return fmt.Sprintf("%s (exit %d): %s", e.Message, e.ExitCode, e.Stderr)
}

// Is makes errors.Is(err, &ShellError{}) work for any *ShellError.
func (e *ShellError) Is(target error) bool {
	_, ok := target.(*ShellError)
	return ok
}

// IsENOENT reports whether err represents a "file not found" error.
// Equivalent to checking for ENOENT on Unix or ERROR_FILE_NOT_FOUND on Windows.
func IsENOENT(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

// IsPermissionDenied reports whether err is a permission-denied error.
func IsPermissionDenied(err error) bool {
	return errors.Is(err, os.ErrPermission)
}

// ErrorMessage safely extracts a string from any value that might be an error.
func ErrorMessage(err interface{}) string {
	if err == nil {
		return ""
	}
	switch v := err.(type) {
	case error:
		return v.Error()
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ToError converts any value to an error, wrapping non-error types as needed.
func ToError(err interface{}) error {
	if err == nil {
		return nil
	}
	if e, ok := err.(error); ok {
		return e
	}
	return errors.New(ErrorMessage(err))
}

// WrapError wraps an error with additional context using %w so errors.Is/As work.
func WrapError(err error, msg string) error {
	return fmt.Errorf("%s: %w", msg, err)
}
