package lsp

import (
	"fmt"

	"github.com/gophersatwork/goverhaul"
	"github.com/sourcegraph/go-lsp"
)

// ViolationToDiagnostic converts a goverhaul violation to an LSP diagnostic
func ViolationToDiagnostic(v goverhaul.LintViolation) lsp.Diagnostic {
	// LSP uses 0-indexed line and column numbers, goverhaul uses 1-indexed
	startLine := 0
	startChar := 0
	endLine := 0
	endChar := 1 // At least highlight one character

	if v.Position != nil && v.Position.IsValid() {
		startLine = v.Position.Line - 1
		startChar = v.Position.Column - 1

		// Use end position if available
		if v.Position.EndLine > 0 {
			endLine = v.Position.EndLine - 1
		} else {
			endLine = startLine
		}

		if v.Position.EndColumn > 0 {
			endChar = v.Position.EndColumn - 1
		} else {
			// If no end position, highlight from start to end of line (reasonable default)
			endChar = startChar + len(v.Import) + 2 // +2 for quotes
		}
	}

	diagnostic := lsp.Diagnostic{
		Range: lsp.Range{
			Start: lsp.Position{
				Line:      startLine,
				Character: startChar,
			},
			End: lsp.Position{
				Line:      endLine,
				Character: endChar,
			},
		},
		Severity: severityToLSP(v.Severity),
		Code:     v.Rule,
		Source:   "goverhaul",
		Message:  formatMessage(v),
	}

	return diagnostic
}

// severityToLSP converts goverhaul severity to LSP severity
func severityToLSP(severity goverhaul.Severity) lsp.DiagnosticSeverity {
	switch severity {
	case goverhaul.SeverityError:
		return lsp.Error // 1
	case goverhaul.SeverityWarning:
		return lsp.Warning // 2
	case goverhaul.SeverityInfo:
		return lsp.Information // 3
	case goverhaul.SeverityHint:
		return lsp.Hint // 4
	default:
		return lsp.Error // Default to error for unknown severity
	}
}

// formatMessage creates a human-readable diagnostic message
func formatMessage(v goverhaul.LintViolation) string {
	msg := fmt.Sprintf("import %q violates rule %q", v.Import, v.Rule)

	if v.Cause != "" {
		msg += fmt.Sprintf(": %s", v.Cause)
	}

	if v.Details != "" {
		msg += fmt.Sprintf(" (%s)", v.Details)
	}

	return msg
}

// ViolationsToDiagnostics converts multiple violations to LSP diagnostics
func ViolationsToDiagnostics(violations []goverhaul.LintViolation) []lsp.Diagnostic {
	diagnostics := make([]lsp.Diagnostic, 0, len(violations))
	for _, v := range violations {
		diagnostics = append(diagnostics, ViolationToDiagnostic(v))
	}
	return diagnostics
}
