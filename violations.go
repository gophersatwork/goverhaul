package goverhaul

import "fmt"

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
