package lsp_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"

	"github.com/gophersatwork/goverhaul"
	"github.com/gophersatwork/goverhaul/lsp"
	"github.com/spf13/afero"
)

// Example demonstrates how to use the LSP code action provider
func Example() {
	// Create an in-memory filesystem for this example
	fs := afero.NewMemMapFs()

	// Create a test Go file with a violation
	goFileContent := `package api

import (
	"fmt"
	"internal/database"
)

func Handler() {
	fmt.Println("test")
}
`
	afero.WriteFile(fs, "/project/internal/api/handler.go", []byte(goFileContent), 0644)

	// Create a configuration file
	configContent := `rules:
  - path: "internal/api"
    allowed:
      - "fmt"
    prohibited:
      - name: "internal/database"
        cause: "Use repository pattern"
`
	afero.WriteFile(fs, "/project/.goverhaul.yml", []byte(configContent), 0644)

	// Create a diagnostic for the violation
	diagnostic := lsp.Diagnostic{
		Range: lsp.Range{
			Start: lsp.Position{Line: 3, Character: 1},
			End:   lsp.Position{Line: 3, Character: 22},
		},
		Severity: lsp.DiagnosticSeverityError,
		Source:   "goverhaul",
		Message:  `import "internal/database" violates rule for path "internal/api": Use repository pattern`,
	}

	// Create the code action provider
	provider := lsp.NewCodeActionProvider(fs, "/project/.goverhaul.yml")

	// Get available code actions
	actions := provider.GetCodeActions(
		"file:///project/internal/api/handler.go",
		diagnostic,
	)

	// Display the available actions
	fmt.Printf("Available quick fixes: %d\n", len(actions))
	for i, action := range actions {
		fmt.Printf("%d. %s (kind: %s)\n", i+1, action.Title, action.Kind)
	}

	// Output:
	// Available quick fixes: 3
	// 1. Remove import "internal/database" (kind: quickfix)
	// 2. Add "internal/database" to allowed list (kind: quickfix)
	// 3. Suppress this violation (kind: quickfix)
}

// Example_server demonstrates how to create and use the LSP server
func Example_server() {
	fs := afero.NewMemMapFs()

	// Setup test environment
	goFileContent := `package main

import "internal/database"

func main() {}
`
	afero.WriteFile(fs, "/project/main.go", []byte(goFileContent), 0644)

	configContent := `rules:
  - path: "."
    prohibited:
      - name: "internal/database"
        cause: "Use repository pattern"
`
	afero.WriteFile(fs, "/project/.goverhaul.yml", []byte(configContent), 0644)
	afero.WriteFile(fs, "/project/go.mod", []byte("module example\n\ngo 1.24\n"), 0644)

	// Load configuration
	cfg, _ := goverhaul.LoadConfig(fs, "/project", "/project/.goverhaul.yml")

	// Create linter
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	linter, _ := goverhaul.NewLinter(cfg, logger, fs)

	// Create LSP server
	var buf bytes.Buffer
	server := lsp.NewServer(linter, &cfg, "/project/.goverhaul.yml", logger, fs, &buf, &buf)

	// Request code actions
	params := lsp.CodeActionParams{
		TextDocument: lsp.TextDocumentIdentifier{
			URI: "file:///project/main.go",
		},
		Range: lsp.Range{
			Start: lsp.Position{Line: 2, Character: 0},
			End:   lsp.Position{Line: 2, Character: 27},
		},
		Context: lsp.CodeActionContext{
			Diagnostics: []lsp.Diagnostic{
				{
					Range: lsp.Range{
						Start: lsp.Position{Line: 2, Character: 0},
						End:   lsp.Position{Line: 2, Character: 27},
					},
					Severity: lsp.DiagnosticSeverityError,
					Source:   "goverhaul",
					Message:  `import "internal/database" violates rule`,
				},
			},
		},
	}

	actions, _ := server.TextDocumentCodeAction(params)

	fmt.Printf("Server returned %d code actions\n", len(actions))
	for _, action := range actions {
		fmt.Printf("- %s\n", action.Title)
	}

	// Note: "Add to allowed list" action may not appear if the rule path
	// cannot be determined from the file path

	// Output:
	// Server returned 2 code actions
	// - Remove import "internal/database"
	// - Suppress this violation
}

// Example_importParser demonstrates import path extraction
func Example_importParser() {
	messages := []string{
		`import "internal/database" violates rule for path "internal/api"`,
		`import "fmt" is prohibited: Use logging`,
		`File contains import "github.com/pkg/errors" which is not allowed`,
	}

	for _, msg := range messages {
		importPath := lsp.ExtractImportPath(msg)
		fmt.Printf("Extracted: %s\n", importPath)
	}

	// Output:
	// Extracted: internal/database
	// Extracted: fmt
	// Extracted: github.com/pkg/errors
}

// Example_configEditor demonstrates programmatic config editing
func Example_configEditor() {
	fs := afero.NewMemMapFs()

	// Create initial config
	initialConfig := `rules:
  - path: "internal/api"
    prohibited:
      - name: "internal/database"
        cause: "Use repository pattern"
`
	configPath := "/project/.goverhaul.yml"
	afero.WriteFile(fs, configPath, []byte(initialConfig), 0644)

	// Create editor
	editor := lsp.NewConfigEditor(fs, configPath)

	// Add import to allowed list
	_, err := editor.AddToAllowedList("internal/api", "encoding/json")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Successfully added 'encoding/json' to allowed list for 'internal/api'")

	// Output:
	// Successfully added 'encoding/json' to allowed list for 'internal/api'
}
