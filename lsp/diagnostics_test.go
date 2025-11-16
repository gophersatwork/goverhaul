package lsp

import (
	"testing"

	"github.com/gophersatwork/goverhaul"
	"github.com/sourcegraph/go-lsp"
	"github.com/stretchr/testify/assert"
)

func TestViolationToDiagnostic(t *testing.T) {
	tests := []struct {
		name       string
		violation  goverhaul.LintViolation
		wantSev    lsp.DiagnosticSeverity
		wantMsg    string
		wantLine   int
		wantChar   int
	}{
		{
			name: "error severity with position",
			violation: goverhaul.LintViolation{
				File:   "test.go",
				Import: "internal/database",
				Rule:   "api-no-db",
				Cause:  "API layer must not access database",
				Severity: goverhaul.SeverityError,
				Position: &goverhaul.Position{
					Line:      5,
					Column:    1,
					EndLine:   5,
					EndColumn: 25,
				},
			},
			wantSev:  lsp.Error,
			wantMsg:  `import "internal/database" violates rule "api-no-db": API layer must not access database`,
			wantLine: 4, // 0-indexed
			wantChar: 0,
		},
		{
			name: "warning severity",
			violation: goverhaul.LintViolation{
				File:     "test.go",
				Import:   "fmt",
				Rule:     "no-fmt",
				Cause:    "Use structured logging instead",
				Severity: goverhaul.SeverityWarning,
				Position: &goverhaul.Position{
					Line:   10,
					Column: 5,
				},
			},
			wantSev:  lsp.Warning,
			wantMsg:  `import "fmt" violates rule "no-fmt": Use structured logging instead`,
			wantLine: 9, // 0-indexed
			wantChar: 4,
		},
		{
			name: "info severity",
			violation: goverhaul.LintViolation{
				File:     "test.go",
				Import:   "internal/utils",
				Rule:     "prefer-pkg",
				Severity: goverhaul.SeverityInfo,
				Position: &goverhaul.Position{
					Line:   3,
					Column: 2,
				},
			},
			wantSev:  lsp.Information,
			wantLine: 2, // 0-indexed
			wantChar: 1,
		},
		{
			name: "hint severity",
			violation: goverhaul.LintViolation{
				File:     "test.go",
				Import:   "internal/old",
				Rule:     "use-new",
				Severity: goverhaul.SeverityHint,
				Position: &goverhaul.Position{
					Line:   7,
					Column: 3,
				},
			},
			wantSev:  lsp.Hint,
			wantLine: 6, // 0-indexed
			wantChar: 2,
		},
		{
			name: "violation without position",
			violation: goverhaul.LintViolation{
				File:     "test.go",
				Import:   "internal/database",
				Rule:     "api-no-db",
				Severity: goverhaul.SeverityError,
			},
			wantSev:  lsp.Error,
			wantLine: 0,
			wantChar: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag := ViolationToDiagnostic(tt.violation)

			assert.Equal(t, tt.wantSev, diag.Severity, "severity mismatch")
			assert.Equal(t, tt.wantLine, diag.Range.Start.Line, "start line mismatch")
			assert.Equal(t, tt.wantChar, diag.Range.Start.Character, "start character mismatch")
			assert.Equal(t, "goverhaul", diag.Source, "source mismatch")
			assert.Equal(t, tt.violation.Rule, diag.Code, "code mismatch")

			if tt.wantMsg != "" {
				assert.Equal(t, tt.wantMsg, diag.Message, "message mismatch")
			}
		})
	}
}

func TestSeverityToLSP(t *testing.T) {
	tests := []struct {
		severity goverhaul.Severity
		want     lsp.DiagnosticSeverity
	}{
		{goverhaul.SeverityError, lsp.Error},
		{goverhaul.SeverityWarning, lsp.Warning},
		{goverhaul.SeverityInfo, lsp.Information},
		{goverhaul.SeverityHint, lsp.Hint},
		{"unknown", lsp.Error}, // Default to error
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			got := severityToLSP(tt.severity)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatMessage(t *testing.T) {
	tests := []struct {
		name      string
		violation goverhaul.LintViolation
		want      string
	}{
		{
			name: "with cause and details",
			violation: goverhaul.LintViolation{
				Import:  "internal/database",
				Rule:    "api-no-db",
				Cause:   "API layer must not access database",
				Details: "This import is explicitly prohibited",
			},
			want: `import "internal/database" violates rule "api-no-db": API layer must not access database (This import is explicitly prohibited)`,
		},
		{
			name: "with cause only",
			violation: goverhaul.LintViolation{
				Import: "fmt",
				Rule:   "no-fmt",
				Cause:  "Use structured logging",
			},
			want: `import "fmt" violates rule "no-fmt": Use structured logging`,
		},
		{
			name: "with details only",
			violation: goverhaul.LintViolation{
				Import:  "internal/utils",
				Rule:    "prefer-pkg",
				Details: "This import is not in the allowed list",
			},
			want: `import "internal/utils" violates rule "prefer-pkg" (This import is not in the allowed list)`,
		},
		{
			name: "minimal",
			violation: goverhaul.LintViolation{
				Import: "internal/old",
				Rule:   "use-new",
			},
			want: `import "internal/old" violates rule "use-new"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMessage(tt.violation)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestViolationsToDiagnostics(t *testing.T) {
	violations := []goverhaul.LintViolation{
		{
			File:     "test.go",
			Import:   "internal/database",
			Rule:     "api-no-db",
			Severity: goverhaul.SeverityError,
			Position: &goverhaul.Position{Line: 5, Column: 1},
		},
		{
			File:     "test.go",
			Import:   "fmt",
			Rule:     "no-fmt",
			Severity: goverhaul.SeverityWarning,
			Position: &goverhaul.Position{Line: 10, Column: 5},
		},
	}

	diagnostics := ViolationsToDiagnostics(violations)

	assert.Len(t, diagnostics, 2)
	assert.Equal(t, int(lsp.Error), int(diagnostics[0].Severity))
	assert.Equal(t, int(lsp.Warning), int(diagnostics[1].Severity))
	assert.Equal(t, "goverhaul", diagnostics[0].Source)
	assert.Equal(t, "goverhaul", diagnostics[1].Source)
}
