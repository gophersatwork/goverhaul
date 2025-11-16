package goverhaul

import "fmt"

// Position represents a location in a source file for IDE integration
type Position struct {
	Line      int `json:"line"`                // 1-indexed line number
	Column    int `json:"column"`              // 1-indexed column number
	Offset    int `json:"offset,omitempty"`    // Byte offset in file
	EndLine   int `json:"end_line,omitempty"`  // For multi-line ranges
	EndColumn int `json:"end_column,omitempty"` // For multi-line ranges
}

// IsValid returns true if the position has valid line/column
func (p *Position) IsValid() bool {
	return p != nil && p.Line > 0 && p.Column > 0
}

// IsSingleLine returns true if the range is within a single line
func (p *Position) IsSingleLine() bool {
	return p != nil && (p.EndLine == 0 || p.Line == p.EndLine)
}

// Severity represents the importance level of a violation
type Severity string

const (
	SeverityError   Severity = "error"   // Blocks commit, shown as error in IDE
	SeverityWarning Severity = "warning" // Warns but allows commit
	SeverityInfo    Severity = "info"    // Informational only
	SeverityHint    Severity = "hint"    // Suggestion for improvement
)

// DefaultSeverity returns the default severity (Error)
func DefaultSeverity() Severity {
	return SeverityError
}

// String implements the Stringer interface for Severity
func (s Severity) String() string {
	return string(s)
}

// ParseSeverity converts a string to a Severity level
// Supports common variations like "warn"/"warning", "info"/"information", "hint"/"suggestion"
// Defaults to SeverityError for backward compatibility if the string is not recognized
func ParseSeverity(s string) Severity {
	switch s {
	case "warning", "warn":
		return SeverityWarning
	case "info", "information":
		return SeverityInfo
	case "hint", "suggestion":
		return SeverityHint
	case "error":
		return SeverityError
	default:
		// Default to error for backward compatibility
		return SeverityError
	}
}

// TextEdit represents a single text modification
type TextEdit struct {
	Range   Position `json:"range"`    // What to replace
	NewText string   `json:"new_text"` // Replacement text
}

// SuggestedFix represents an automated fix for a violation
type SuggestedFix struct {
	Title       string     `json:"title"`                  // "Remove import", "Add to allowed list"
	Edits       []TextEdit `json:"edits"`                  // File modifications
	IsPreferred bool       `json:"is_preferred,omitempty"` // Auto-apply if true
}

// RelatedInfo provides additional context for a violation
type RelatedInfo struct {
	Location Location `json:"location"` // Related file/position
	Message  string   `json:"message"`  // Explanation
}

// Location represents a position in a file
type Location struct {
	FilePath string   `json:"file_path"`
	Range    Position `json:"range"`
}

// LintViolation represents a specific rule violation found during linting
type LintViolation struct {
	// Existing fields
	File    string `json:"file"`    // The file where the violation was found
	Import  string `json:"import"`  // The import that violated the rule
	Rule    string `json:"rule"`    // The rule that was violated
	Cause   string `json:"cause"`   // The cause of the violation, if provided
	Details string `json:"details"` // Additional details about the violation
	Cached  bool   `json:"cached"`  // Whether the lint violation result was retrieved from the cache.

	// NEW: Position information for IDE integration
	Position     *Position      `json:"position,omitempty"`      // Where the violation occurs
	Severity     Severity       `json:"severity,omitempty"`      // Error, Warning, Info, Hint
	SuggestedFix *SuggestedFix  `json:"suggested_fix,omitempty"` // Automated fix
	RelatedInfo  []RelatedInfo  `json:"related_info,omitempty"`  // Context

	// NEW: Metadata for UX
	RuleDoc  string `json:"rule_doc,omitempty"`  // Link to documentation
	Category string `json:"category,omitempty"`  // "dependency", "layer-violation"
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

// String implements the Stringer interface
func (v *LintViolations) String() string {
	return v.PrintByFile()
}

// PrintByFile prints the violations grouped by file
func (v *LintViolations) PrintByFile() string {
	if len(v.Violations) == 0 {
		return "No rule violations found"
	}

	msg := fmt.Sprintf("Found %d rule violations grouped by file:\n", len(v.Violations))

	// Group violations by file
	fileViolations := make(map[string][]LintViolation)
	for _, violation := range v.Violations {
		fileViolations[violation.File] = append(fileViolations[violation.File], violation)
	}

	// Display violations for each file
	for file, violations := range fileViolations {
		msg += fmt.Sprintf("File: %s (%d violations)\n", file, len(violations))

		for _, violation := range violations {
			if violation.Cause != "" {
				msg += fmt.Sprintf("  - Rule: %s, Import: %s, Cause: %s\n", violation.Rule, violation.Import, violation.Cause)
			} else {
				msg += fmt.Sprintf("  - Rule: %s, Import: %s\n", violation.Rule, violation.Import)
			}
		}
		msg += "\n"
	}

	return msg
}

// PrintByRule prints the violations grouped by rule
func (v *LintViolations) PrintByRule() string {
	if len(v.Violations) == 0 {
		return "No rule violations found"
	}

	msg := fmt.Sprintf("Found %d rule violations grouped by rule:\n", len(v.Violations))

	// Group violations by rule
	ruleViolations := make(map[string][]LintViolation)
	for _, violation := range v.Violations {
		ruleViolations[violation.Rule] = append(ruleViolations[violation.Rule], violation)
	}

	// Display violations for each rule
	for rule, violations := range ruleViolations {
		msg += fmt.Sprintf("Rule: %s (%d violations)\n", rule, len(violations))

		for _, violation := range violations {
			if violation.Cause != "" {
				msg += fmt.Sprintf("  - File: %s, Import: %s, Cause: %s\n", violation.File, violation.Import, violation.Cause)
			} else {
				msg += fmt.Sprintf("  - File: %s, Import: %s\n", violation.File, violation.Import)
			}
		}
		msg += "\n"
	}

	return msg
}
