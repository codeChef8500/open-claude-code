package util

import (
	"errors"
	"fmt"
	"os"
	"strings"
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

// ErrorCode categorizes application-level errors.
type ErrorCode string

const (
	ErrCodeConfig       ErrorCode = "config_error"
	ErrCodeProvider     ErrorCode = "provider_error"
	ErrCodePermission   ErrorCode = "permission_error"
	ErrCodeTool         ErrorCode = "tool_error"
	ErrCodeSession      ErrorCode = "session_error"
	ErrCodeCompaction   ErrorCode = "compaction_error"
	ErrCodeHook         ErrorCode = "hook_error"
	ErrCodeMCP          ErrorCode = "mcp_error"
	ErrCodeMemory       ErrorCode = "memory_error"
	ErrCodeInput        ErrorCode = "input_error"
	ErrCodeInternal     ErrorCode = "internal_error"
	ErrCodeCancelled    ErrorCode = "cancelled"
	ErrCodeTimeout      ErrorCode = "timeout"
	ErrCodeCostLimit    ErrorCode = "cost_limit"
	ErrCodeContextLimit ErrorCode = "context_limit"
)

// AppError is a structured application error with code, message, and cause.
type AppError struct {
	Code    ErrorCode
	Message string
	Cause   error
	Details map[string]string
}

// NewAppError creates a new AppError.
func NewAppError(code ErrorCode, msg string, cause error) *AppError {
	return &AppError{Code: code, Message: msg, Cause: cause}
}

// WithDetail adds a key-value detail to the error.
func (e *AppError) WithDetail(key, value string) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]string)
	}
	e.Details[key] = value
	return e
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Cause }

// IsAppError checks if err is an AppError with the given code.
func IsAppError(err error, code ErrorCode) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}

// AppErrorCode extracts the ErrorCode from an error, or ErrCodeInternal if not an AppError.
func AppErrorCode(err error) ErrorCode {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ErrCodeInternal
}

// UserFacingMessage returns a user-friendly error message.
func UserFacingMessage(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	var abortErr *AbortError
	if errors.As(err, &abortErr) {
		return abortErr.Error()
	}
	// Strip internal details for unknown errors.
	msg := err.Error()
	if len(msg) > 200 {
		return msg[:200] + "..."
	}
	return msg
}

// MultiError collects multiple errors into one.
type MultiError struct {
	Errors []error
}

func (e *MultiError) Error() string {
	if len(e.Errors) == 0 {
		return "no errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	var parts []string
	for _, err := range e.Errors {
		parts = append(parts, err.Error())
	}
	return fmt.Sprintf("%d errors: %s", len(e.Errors), strings.Join(parts, "; "))
}

// Add appends an error (nil errors are ignored).
func (e *MultiError) Add(err error) {
	if err != nil {
		e.Errors = append(e.Errors, err)
	}
}

// Err returns nil if no errors were collected, or the MultiError.
func (e *MultiError) Err() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e
}
