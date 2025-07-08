package main

import (
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

// AppError is a custom error type that provides context about the error
type AppError struct {
	Type    ErrorType // The category of the error
	Message string    // A human-readable error message
	Err     error     // The underlying error, if any
	File    string    // The file related to the error, if applicable
	Details string    // Additional details about the error
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

// WithFile adds file information to the error
func (e *AppError) WithFile(file string) *AppError {
	e.File = file
	return e
}

// WithDetails adds additional details to the error
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// NewConfigError creates a new configuration error
func NewConfigError(message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeConfig,
		Message: message,
		Err:     err,
	}
}

// NewFSError creates a new file system error
func NewFSError(message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeFS,
		Message: message,
		Err:     err,
	}
}

// NewParseError creates a new parsing error
func NewParseError(message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeParse,
		Message: message,
		Err:     err,
	}
}

// NewLintError creates a new linting error
func NewLintError(message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeLint,
		Message: message,
		Err:     err,
	}
}

// NewCacheError creates a new cache error
func NewCacheError(message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeCache,
		Message: message,
		Err:     err,
	}
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