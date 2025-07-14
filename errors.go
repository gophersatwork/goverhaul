package goverhaul

import (
	"errors"
	"fmt"
)

// ErrorType represents the category of an error
type ErrorType string

const (
	// ErrorTypeConfig represents configuration-related errors
	ErrorTypeConfig ErrorType = "config"
	// ErrorTypeFS represents file system-related errors
	ErrorTypeFS ErrorType = "filesystem"
	// ErrorTypeParse represents parsing-related errors
	ErrorTypeParse ErrorType = "parse"
	// ErrorTypeLint represents linting-related errors
	ErrorTypeLint ErrorType = "lint"
	// ErrorTypeCache represents cache-related errors
	ErrorTypeCache ErrorType = "cache"
)

// AppError represents an application error with additional context
type AppError struct {
	Type    ErrorType // Type of error
	Message string    // Human-readable error message
	File    string    // Optional: file path related to the error
	Details string    // Optional: additional details about the error
	Err     error     // Optional: wrapped error
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// GetErrorInfo extracts error information from an error chain
func GetErrorInfo(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}

	return nil, false
}

// WithFile adds file information to an error
func WithFile(err error, file string) error {
	var appErr *AppError
	if errors.As(err, &appErr) {
		// Create a new AppError with the file information
		return &AppError{
			Type:    appErr.Type,
			Message: appErr.Message,
			File:    file,
			Details: appErr.Details,
			Err:     appErr.Err,
		}
	}

	// If it's not an AppError, wrap it in a new AppError
	return &AppError{
		Type:    ErrorTypeFS, // Default type
		Message: err.Error(),
		File:    file,
		Err:     err,
	}
}

// WithDetails adds additional details to an error
func WithDetails(err error, details string) error {
	var appErr *AppError
	if errors.As(err, &appErr) {
		// Create a new AppError with the details information
		return &AppError{
			Type:    appErr.Type,
			Message: appErr.Message,
			File:    appErr.File,
			Details: details,
			Err:     appErr.Err,
		}
	}

	// If it's not an AppError, wrap it in a new AppError
	return &AppError{
		Type:    ErrorTypeFS, // Default type
		Message: err.Error(),
		Details: details,
		Err:     err,
	}
}

// NewError creates a new application error with the specified type
func NewError(errType ErrorType, message string, err error) error {
	return &AppError{
		Type:    errType,
		Message: message,
		Err:     err,
	}
}

// NewConfigError creates a new configuration error
func NewConfigError(message string, err error) error {
	return NewError(ErrorTypeConfig, message, err)
}

// NewFSError creates a new file system error
func NewFSError(message string, err error) error {
	return NewError(ErrorTypeFS, message, err)
}

// NewParseError creates a new parsing error
func NewParseError(message string, err error) error {
	return NewError(ErrorTypeParse, message, err)
}

// NewLintError creates a new linting error
func NewLintError(message string, err error) error {
	return NewError(ErrorTypeLint, message, err)
}

// NewCacheError creates a new cache error
func NewCacheError(message string, err error) error {
	return NewError(ErrorTypeCache, message, err)
}
