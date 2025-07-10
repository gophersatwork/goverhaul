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

// LintViolation represents a specific rule violation found during linting
type LintViolation struct {
	File    string `json:"file"`    // The file where the violation was found
	Import  string `json:"import"`  // The import that violated the rule
	Rule    string `json:"rule"`    // The rule that was violated
	Cause   string `json:"cause"`   // The cause of the violation, if provided
	Details string `json:"details"` // Additional details about the violation
	Cached  bool   `json:"cached"`  // Whether the lint violation result was retrieved from the cache.
}

// Error implements the error interface
func (v *LintViolation) Error() string {
	if v.Cause != "" {
		return fmt.Sprintf("Rule violation in %s: import %s is not allowed (%s)", v.File, v.Import, v.Cause)
	}
	return fmt.Sprintf("Rule violation in %s: import %s is not allowed", v.File, v.Import)
}

// LintViolations is a collection of LintViolation errors
type LintViolations struct {
	Violations []LintViolation `json:"violations"`
}

// Error implements the error interface
func (v *LintViolations) Error() string {
	if len(v.Violations) == 0 {
		return "No rule violations found"
	}

	msg := fmt.Sprintf("Found %d rule violations categorized by rule:\n", len(v.Violations))

	// Group violations by rule
	ruleViolations := make(map[string][]LintViolation)
	for _, violation := range v.Violations {
		ruleViolations[violation.Rule] = append(ruleViolations[violation.Rule], violation)
	}

	// Display violations grouped by rule
	for rule, violations := range ruleViolations {
		msg += fmt.Sprintf("Rule: %s (%d violations)\n", rule, len(violations))

		// Track files to avoid duplicates within each rule
		files := make(map[string]bool)

		for _, violation := range violations {
			if !files[violation.File] {
				files[violation.File] = true
				msg += fmt.Sprintf("  - %s\n", violation.File)
			}
		}
	}

	return msg
}

// Add adds a violation to the collection
func (v *LintViolations) Add(violation LintViolation) {
	v.Violations = append(v.Violations, violation)
}

// NewLintViolations creates a new empty collection of lint violations
func NewLintViolations() *LintViolations {
	return &LintViolations{
		Violations: make([]LintViolation, 0),
	}
}

// IsEmpty returns true if there are no violations
func (v *LintViolations) IsEmpty() bool {
	return len(v.Violations) == 0
}
