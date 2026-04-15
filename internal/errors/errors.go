package errors

import (
	"fmt"
	"os"
)

// Exit codes as defined in FR-006
const (
	ExitSuccess         = 0
	ExitConfigError     = 1
	ExitAuthError       = 2
	ExitNetworkError    = 3
	ExitFileSystemError = 4
	ExitTokenError      = 5
)

// AppError represents an application error with an exit code
type AppError struct {
	Message  string
	ExitCode int
	Cause    error
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("Error: %s: %v", e.Message, e.Cause)
	}
	return fmt.Sprintf("Error: %s", e.Message)
}

// NewConfigError creates a configuration error (exit code 1)
func NewConfigError(message string, cause error) *AppError {
	return &AppError{
		Message:  message,
		ExitCode: ExitConfigError,
		Cause:    cause,
	}
}

// NewAuthError creates an authentication error (exit code 2)
func NewAuthError(message string, cause error) *AppError {
	return &AppError{
		Message:  message,
		ExitCode: ExitAuthError,
		Cause:    cause,
	}
}

// NewNetworkError creates a network error (exit code 3)
func NewNetworkError(message string, cause error) *AppError {
	return &AppError{
		Message:  message,
		ExitCode: ExitNetworkError,
		Cause:    cause,
	}
}

// NewFileSystemError creates a file system error (exit code 4)
func NewFileSystemError(message string, cause error) *AppError {
	return &AppError{
		Message:  message,
		ExitCode: ExitFileSystemError,
		Cause:    cause,
	}
}

// NewTokenError creates a token error (exit code 5)
func NewTokenError(message string, cause error) *AppError {
	return &AppError{
		Message:  message,
		ExitCode: ExitTokenError,
		Cause:    cause,
	}
}

// Fatal prints the error to stderr and exits with the appropriate code
func Fatal(err error) {
	if appErr, ok := err.(*AppError); ok {
		fmt.Fprintln(os.Stderr, appErr.Error())
		os.Exit(appErr.ExitCode)
	}
	// Fallback for non-AppError errors
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

// SanitizeError removes sensitive data from error messages
// Sensitive data includes: access_token, refresh_token, client_secret, authorization codes
func SanitizeError(err error) error {
	if err == nil {
		return nil
	}
	// For now, just return the error as-is
	// In future stories, we'll add pattern matching to redact sensitive data
	return err
}
