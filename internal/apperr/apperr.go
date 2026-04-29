// Package apperr defines the application's error types and exit codes.
package apperr

import (
	"errors"
	"fmt"
	"os"
)

// Exit codes returned by the binary.
const (
	ExitSuccess         = 0
	ExitConfigError     = 1
	ExitAuthError       = 2
	ExitNetworkError    = 3
	ExitFileSystemError = 4
	ExitTokenError      = 5
)

// Domain sentinel errors. Use errors.Is to match.
var (
	ErrTokenFileNotFound  = errors.New("token file not found")
	ErrInvalidTokenFormat = errors.New("invalid token file format")
	ErrTokenMissingField  = errors.New("token file missing required field")
)

// AppError is the application's typed error carrying a user-facing message,
// an exit code, and an optional underlying cause.
type AppError struct {
	Message  string
	ExitCode int
	Cause    error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("Error: %s: %v", e.Message, e.Cause)
	}
	return fmt.Sprintf("Error: %s", e.Message)
}

// Unwrap exposes the underlying cause for errors.Is / errors.As.
func (e *AppError) Unwrap() error {
	return e.Cause
}

// NewConfigError returns a configuration error (exit code 1).
func NewConfigError(message string, cause error) *AppError {
	return &AppError{Message: message, ExitCode: ExitConfigError, Cause: cause}
}

// NewAuthError returns an authentication error (exit code 2).
func NewAuthError(message string, cause error) *AppError {
	return &AppError{Message: message, ExitCode: ExitAuthError, Cause: cause}
}

// NewNetworkError returns a network error (exit code 3).
func NewNetworkError(message string, cause error) *AppError {
	return &AppError{Message: message, ExitCode: ExitNetworkError, Cause: cause}
}

// NewFileSystemError returns a file system error (exit code 4).
func NewFileSystemError(message string, cause error) *AppError {
	return &AppError{Message: message, ExitCode: ExitFileSystemError, Cause: cause}
}

// NewTokenError returns a token error (exit code 5).
func NewTokenError(message string, cause error) *AppError {
	return &AppError{Message: message, ExitCode: ExitTokenError, Cause: cause}
}

// Fatal prints err to stderr and exits with the matching code.
func Fatal(err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		fmt.Fprintln(os.Stderr, appErr.Error())
		os.Exit(appErr.ExitCode)
	}
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
