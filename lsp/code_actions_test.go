package lsp

import (
	"testing"

	"github.com/gophersatwork/goverhaul"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodeActionProvider_GetCodeActions(t *testing.T) {
	tests := []struct {
		name            string
		setupFS         func(afero.Fs)
		diagnostic      Diagnostic
		uri             string
		expectedActions int
		checkActions    func(*testing.T, []CodeAction)
	}{
		{
			name: "provides remove import action without config",
			setupFS: func(fs afero.Fs) {
				// Create a test Go file with an import
				content := `package main

import (
	"fmt"
	"internal/database"
)

func main() {
	fmt.Println("hello")
}
`
				afero.WriteFile(fs, "/test/main.go", []byte(content), 0644)
			},
			diagnostic: Diagnostic{
				Range: Range{
					Start: Position{Line: 4, Character: 1},
					End:   Position{Line: 4, Character: 22},
				},
				Severity: DiagnosticSeverityError,
				Source:   "goverhaul",
				Message:  `import "internal/database" violates rule`,
			},
			uri:             "file:///test/main.go",
			expectedActions: 2, // remove, suppress (no config, so no "add to allowed")
			checkActions: func(t *testing.T, actions []CodeAction) {
				// Check that we have a remove action
				var removeAction *CodeAction
				for i := range actions {
					if actions[i].Kind == QuickFix && actions[i].Title == `Remove import "internal/database"` {
						removeAction = &actions[i]
						break
					}
				}
				require.NotNil(t, removeAction, "Remove action should be present")
				assert.Equal(t, QuickFix, removeAction.Kind)
				assert.NotNil(t, removeAction.Edit)
				assert.Contains(t, removeAction.Edit.Changes, "file:///test/main.go")
			},
		},
		{
			name: "provides add to allowed action with config",
			setupFS: func(fs afero.Fs) {
				// Create a test Go file
				content := `package main

import "internal/database"

func main() {}
`
				afero.WriteFile(fs, "/test/main.go", []byte(content), 0644)

				// Create a config file
				configContent := `rules:
  - path: "test"
    prohibited:
      - name: "internal/database"
        cause: "Use repository pattern"
`
				afero.WriteFile(fs, "/test/.goverhaul.yml", []byte(configContent), 0644)
			},
			diagnostic: Diagnostic{
				Range: Range{
					Start: Position{Line: 2, Character: 0},
					End:   Position{Line: 2, Character: 27},
				},
				Severity: DiagnosticSeverityError,
				Source:   "goverhaul",
				Message:  `import "internal/database" violates rule`,
			},
			uri:             "file:///test/main.go",
			expectedActions: 3,
			checkActions: func(t *testing.T, actions []CodeAction) {
				// Check that we have an add to allowed action
				var addAction *CodeAction
				for i := range actions {
					if actions[i].Kind == QuickFix && actions[i].Title == `Add "internal/database" to allowed list` {
						addAction = &actions[i]
						break
					}
				}
				require.NotNil(t, addAction, "Add to allowed action should be present")
				assert.True(t, addAction.IsPreferred)
				assert.NotNil(t, addAction.Edit)
			},
		},
		{
			name: "provides suppress action",
			setupFS: func(fs afero.Fs) {
				// Create a test Go file
				content := `package main

import "internal/database"

func main() {}
`
				afero.WriteFile(fs, "/test/main.go", []byte(content), 0644)
			},
			diagnostic: Diagnostic{
				Range: Range{
					Start: Position{Line: 2, Character: 0},
					End:   Position{Line: 2, Character: 27},
				},
				Severity: DiagnosticSeverityError,
				Source:   "goverhaul",
				Message:  `import "internal/database" violates rule`,
			},
			uri:             "file:///test/main.go",
			expectedActions: 2, // remove, suppress (no config file)
			checkActions: func(t *testing.T, actions []CodeAction) {
				// Check that we have a suppress action
				var suppressAction *CodeAction
				for i := range actions {
					if actions[i].Kind == QuickFix && actions[i].Title == "Suppress this violation" {
						suppressAction = &actions[i]
						break
					}
				}
				require.NotNil(t, suppressAction, "Suppress action should be present")
				assert.NotNil(t, suppressAction.Edit)
				edits := suppressAction.Edit.Changes["file:///test/main.go"]
				require.Len(t, edits, 1)
				assert.Contains(t, edits[0].NewText, "goverhaul:ignore")
			},
		},
		{
			name: "returns empty for non-goverhaul diagnostic",
			setupFS: func(fs afero.Fs) {
				content := `package main

import "fmt"

func main() {}
`
				afero.WriteFile(fs, "/test/main.go", []byte(content), 0644)
			},
			diagnostic: Diagnostic{
				Range:    Range{Start: Position{Line: 2, Character: 0}, End: Position{Line: 2, Character: 12}},
				Severity: DiagnosticSeverityError,
				Source:   "other-linter",
				Message:  "some other error",
			},
			uri:             "file:///test/main.go",
			expectedActions: 0,
			checkActions: func(t *testing.T, actions []CodeAction) {
				assert.Empty(t, actions)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			tt.setupFS(fs)

			provider := NewCodeActionProvider(fs, "/test/.goverhaul.yml")
			actions := provider.GetCodeActions(tt.uri, tt.diagnostic)

			assert.Len(t, actions, tt.expectedActions)
			if tt.checkActions != nil {
				tt.checkActions(t, actions)
			}
		})
	}
}

func TestCodeActionProvider_createRemoveImportAction(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create a test file with imports
	content := `package main

import (
	"fmt"
	"internal/database"
	"strings"
)

func main() {
	fmt.Println("hello")
}
`
	afero.WriteFile(fs, "/test/main.go", []byte(content), 0644)

	provider := NewCodeActionProvider(fs, "")
	diagnostic := Diagnostic{
		Range:    Range{Start: Position{Line: 4, Character: 1}, End: Position{Line: 4, Character: 22}},
		Severity: DiagnosticSeverityError,
		Source:   "goverhaul",
		Message:  `import "internal/database" violates rule`,
	}

	action := provider.createRemoveImportAction(
		"file:///test/main.go",
		"/test/main.go",
		diagnostic,
		"internal/database",
	)

	require.NotNil(t, action)
	assert.Equal(t, `Remove import "internal/database"`, action.Title)
	assert.Equal(t, QuickFix, action.Kind)
	assert.NotNil(t, action.Edit)

	edits := action.Edit.Changes["file:///test/main.go"]
	require.Len(t, edits, 1)
	assert.Equal(t, "", edits[0].NewText) // Should be empty to delete
}

func TestCodeActionProvider_createSuppressAction(t *testing.T) {
	fs := afero.NewMemMapFs()

	content := `package main

import "internal/database"

func main() {}
`
	afero.WriteFile(fs, "/test/main.go", []byte(content), 0644)

	provider := NewCodeActionProvider(fs, "")
	diagnostic := Diagnostic{
		Range:    Range{Start: Position{Line: 2, Character: 0}, End: Position{Line: 2, Character: 27}},
		Severity: DiagnosticSeverityError,
		Source:   "goverhaul",
		Message:  `import "internal/database" violates rule`,
	}

	action := provider.createSuppressAction(
		"file:///test/main.go",
		"/test/main.go",
		diagnostic,
		"internal/database",
	)

	require.NotNil(t, action)
	assert.Equal(t, "Suppress this violation", action.Title)
	assert.Equal(t, QuickFix, action.Kind)

	edits := action.Edit.Changes["file:///test/main.go"]
	require.Len(t, edits, 1)
	assert.Contains(t, edits[0].NewText, "goverhaul:ignore")
}

func TestCodeActionProvider_findApplicableRule(t *testing.T) {
	fs := afero.NewMemMapFs()
	provider := NewCodeActionProvider(fs, "")

	config := goverhaul.Config{
		Rules: []goverhaul.Rule{
			{
				Path: "internal/api",
				Allowed: []string{"fmt", "strings"},
			},
			{
				Path: "internal/database",
				Prohibited: []goverhaul.ProhibitedPkg{
					{Name: "fmt", Cause: "Use logging"},
				},
			},
		},
	}

	tests := []struct {
		name         string
		filePath     string
		expectedRule string
	}{
		{
			name:         "matches api rule",
			filePath:     "/project/internal/api/handler.go",
			expectedRule: "internal/api",
		},
		{
			name:         "matches database rule",
			filePath:     "/project/internal/database/db.go",
			expectedRule: "internal/database",
		},
		{
			name:         "no match",
			filePath:     "/project/cmd/main.go",
			expectedRule: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := provider.findApplicableRule(config, tt.filePath)
			assert.Equal(t, tt.expectedRule, rule)
		})
	}
}
