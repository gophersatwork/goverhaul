package lsp

import (
	"bytes"
	"log/slog"
	"os"
	"testing"

	"github.com/gophersatwork/goverhaul"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_TextDocumentCodeAction(t *testing.T) {
	tests := []struct {
		name           string
		setupFS        func(afero.Fs) (string, string) // returns filePath and configPath
		params         CodeActionParams
		expectedCount  int
		checkActions   func(*testing.T, []CodeAction)
	}{
		{
			name: "returns actions for goverhaul diagnostics",
			setupFS: func(fs afero.Fs) (string, string) {
				filePath := "/test/internal/api/handler.go"
				content := `package api

import (
	"fmt"
	"internal/database"
)

func Handler() {
	fmt.Println("test")
}
`
				afero.WriteFile(fs, filePath, []byte(content), 0644)

				configPath := "/test/.goverhaul.yml"
				configContent := `rules:
  - path: "internal/api"
    allowed:
      - "fmt"
    prohibited:
      - name: "internal/database"
        cause: "Use repository pattern"
`
				afero.WriteFile(fs, configPath, []byte(configContent), 0644)

				// Create go.mod
				afero.WriteFile(fs, "/test/go.mod", []byte("module test\n\ngo 1.24\n"), 0644)

				return filePath, configPath
			},
			params: CodeActionParams{
				TextDocument: TextDocumentIdentifier{
					URI: "file:///test/internal/api/handler.go",
				},
				Range: Range{
					Start: Position{Line: 4, Character: 0},
					End:   Position{Line: 4, Character: 25},
				},
				Context: CodeActionContext{
					Diagnostics: []Diagnostic{
						{
							Range: Range{
								Start: Position{Line: 4, Character: 1},
								End:   Position{Line: 4, Character: 22},
							},
							Severity: DiagnosticSeverityError,
							Source:   "goverhaul",
							Message:  `import "internal/database" violates rule`,
						},
					},
				},
			},
			expectedCount: 3, // remove, add to allowed, suppress
			checkActions: func(t *testing.T, actions []CodeAction) {
				titles := make([]string, len(actions))
				for i, action := range actions {
					titles[i] = action.Title
				}

				assert.Contains(t, titles, `Remove import "internal/database"`)
				assert.Contains(t, titles, `Add "internal/database" to allowed list`)
				assert.Contains(t, titles, "Suppress this violation")
			},
		},
		{
			name: "returns empty for non-goverhaul diagnostics",
			setupFS: func(fs afero.Fs) (string, string) {
				filePath := "/test/main.go"
				afero.WriteFile(fs, filePath, []byte("package main\n"), 0644)
				return filePath, ""
			},
			params: CodeActionParams{
				TextDocument: TextDocumentIdentifier{
					URI: "file:///test/main.go",
				},
				Range: Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: 0}},
				Context: CodeActionContext{
					Diagnostics: []Diagnostic{
						{
							Range:    Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: 0}},
							Severity: DiagnosticSeverityError,
							Source:   "other-linter",
							Message:  "some other error",
						},
					},
				},
			},
			expectedCount: 0,
			checkActions: func(t *testing.T, actions []CodeAction) {
				assert.Empty(t, actions)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			filePath, configPath := tt.setupFS(fs)

			// Create a linter
			cfg, err := goverhaul.LoadConfig(fs, "/test", configPath)
			if err != nil && configPath != "" {
				t.Fatalf("Failed to load config: %v", err)
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			linter, err := goverhaul.NewLinter(cfg, logger, fs)
			require.NoError(t, err)

			// Create server
			var buf bytes.Buffer
			server := NewServer(linter, &cfg, configPath, logger, fs, &buf, &buf)

			// Call TextDocumentCodeAction
			actions, err := server.TextDocumentCodeAction(tt.params)

			require.NoError(t, err)
			assert.Len(t, actions, tt.expectedCount)

			if tt.checkActions != nil {
				tt.checkActions(t, actions)
			}

			_ = filePath // Use the variable
		})
	}
}

func TestServer_violationsToDiagnostics(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create a test file
	filePath := "/test/main.go"
	content := `package main

import "internal/database"

func main() {}
`
	afero.WriteFile(fs, filePath, []byte(content), 0644)

	violations := &goverhaul.LintViolations{
		Violations: []goverhaul.LintViolation{
			{
				File:    filePath,
				Import:  "internal/database",
				Rule:    "test",
				Cause:   "Use repository pattern",
				Details: "This import violates architectural boundaries",
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := goverhaul.Config{}
	linter, _ := goverhaul.NewLinter(cfg, logger, fs)

	var buf bytes.Buffer
	server := NewServer(linter, &cfg, "", logger, fs, &buf, &buf)

	diagnostics := server.violationsToDiagnostics(violations, filePath)

	require.Len(t, diagnostics, 1)
	assert.Equal(t, "goverhaul", diagnostics[0].Source)
	assert.Equal(t, DiagnosticSeverityError, diagnostics[0].Severity)
	assert.Contains(t, diagnostics[0].Message, "internal/database")
	assert.Contains(t, diagnostics[0].Message, "Use repository pattern")
}

func TestServer_violationsToDiagnostics_EmptyViolations(t *testing.T) {
	fs := afero.NewMemMapFs()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := goverhaul.Config{}
	linter, _ := goverhaul.NewLinter(cfg, logger, fs)

	var buf bytes.Buffer
	server := NewServer(linter, &cfg, "", logger, fs, &buf, &buf)

	diagnostics := server.violationsToDiagnostics(nil, "/test/main.go")
	assert.Empty(t, diagnostics)

	emptyViolations := &goverhaul.LintViolations{
		Violations: []goverhaul.LintViolation{},
	}
	diagnostics = server.violationsToDiagnostics(emptyViolations, "/test/main.go")
	assert.Empty(t, diagnostics)
}

func TestServer_violationsToDiagnostics_FiltersOtherFiles(t *testing.T) {
	fs := afero.NewMemMapFs()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := goverhaul.Config{}
	linter, _ := goverhaul.NewLinter(cfg, logger, fs)

	violations := &goverhaul.LintViolations{
		Violations: []goverhaul.LintViolation{
			{
				File:   "/test/other.go",
				Import: "fmt",
				Rule:   "test",
			},
			{
				File:   "/test/main.go",
				Import: "strings",
				Rule:   "test",
			},
		},
	}

	var buf bytes.Buffer
	server := NewServer(linter, &cfg, "", logger, fs, &buf, &buf)

	// Should only return diagnostics for /test/main.go
	diagnostics := server.violationsToDiagnostics(violations, "/test/main.go")
	require.Len(t, diagnostics, 1)
	assert.Contains(t, diagnostics[0].Message, "strings")
}
