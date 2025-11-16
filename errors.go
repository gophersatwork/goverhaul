package goverhaul

import (
	"errors"
	"fmt"
)

// AppError represents an application error with additional context
type AppError struct {
	Message string
	File    string
	Details string
	Err     error
}

// Error implements the error interface
func (e *AppError) Error() string {
	var result string

	// Add message
	result = e.Message

	// Add wrapped error message if available
	if e.Err != nil && e.Err.Error() != e.Message {
		result += ": " + e.Err.Error()
	}

	// Add file and details if available
	if e.File != "" && e.Details != "" {
		result += fmt.Sprintf(" (%s: %s)", e.File, e.Details)
	} else if e.File != "" {
		result += fmt.Sprintf(" (%s)", e.File)
	} else if e.Details != "" {
		result += fmt.Sprintf(" (%s)", e.Details)
	}

	return result
}

// Unwrap returns the wrapped error
func (e *AppError) Unwrap() error {
	return e.Err
}

// GetErrorInfo extracts error information from an error
func GetErrorInfo(err error) (*AppError, bool) {
	if err == nil {
		return nil, false
	}

	// Check if the error is already an AppError
	if ae, ok := err.(*AppError); ok {
		return ae, true
	}

	return nil, false
}

// WithFile adds file information to an error
func WithFile(err error, file string) error {
	if err == nil {
		return nil
	}

	// If it's already an AppError, add the file info
	if appErr, ok := GetErrorInfo(err); ok {
		appErr.File = file
		return appErr
	}

	// Otherwise, create a new AppError
	return &AppError{
		Message: err.Error(),
		File:    file,
		Err:     err,
	}
}

// WithDetails adds additional details to an error
func WithDetails(err error, details string) error {
	if err == nil {
		return nil
	}

	// If it's already an AppError, add the details
	if appErr, ok := GetErrorInfo(err); ok {
		appErr.Details = details
		return appErr
	}

	// Otherwise, create a new AppError
	return &AppError{
		Message: err.Error(),
		Details: details,
		Err:     err,
	}
}

// NewError creates a new application error
func NewError(message string, err error) error {
	return &AppError{
		Message: message,
		Err:     err,
	}
}

// NewConfigError creates a new configuration error
func NewConfigError(message string, err error) error {
	return NewError(message, err)
}

// NewFSError creates a new file system error
func NewFSError(message string, err error) error {
	return NewError(message, err)
}

// NewParseError creates a new parsing error
func NewParseError(message string, err error) error {
	return NewError(message, err)
}

// NewLintError creates a new linting error
func NewLintError(message string, err error) error {
	return NewError(message, err)
}

// NewCacheError creates a new cache error
func NewCacheError(message string, err error) error {
	return NewError(message, err)
}

// Cache-related errors
var (
	ErrEntryNotFound           = errors.New("entry not found")
	ErrReadingCachedViolations = errors.New("cached violations are invalid")
)
