package lsp

import (
	"log/slog"
	"os"
	"testing"

	"github.com/gophersatwork/goverhaul"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHoverProvider_FindImportAtPosition(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create a test Go file with imports
	testFile := `package main

import (
	"context"
	"fmt"
	"internal/database"
)

func main() {
	fmt.Println("hello")
}
`
	err := afero.WriteFile(fs, "test/main.go", []byte(testFile), 0644)
	require.NoError(t, err)

	config := &goverhaul.Config{
		Rules: []goverhaul.Rule{},
	}

	linter, err := goverhaul.NewLinter(*config, slog.New(slog.NewTextHandler(os.Stdout, nil)), fs)
	require.NoError(t, err)

	provider := NewHoverProvider(linter, config, fs)

	tests := []struct {
		name         string
		position     Position
		wantImport   string
		wantNotEmpty bool
	}{
		{
			name:         "position on context import",
			position:     Position{Line: 3, Character: 5},
			wantImport:   "context",
			wantNotEmpty: true,
		},
		{
			name:         "position on fmt import",
			position:     Position{Line: 4, Character: 5},
			wantImport:   "fmt",
			wantNotEmpty: true,
		},
		{
			name:         "position on internal/database import",
			position:     Position{Line: 5, Character: 5},
			wantImport:   "internal/database",
			wantNotEmpty: true,
		},
		{
			name:         "position not on import",
			position:     Position{Line: 8, Character: 0},
			wantImport:   "",
			wantNotEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			importPath, importRange := provider.findImportAtPosition("file:///test/main.go", tt.position)

			if tt.wantNotEmpty {
				assert.Equal(t, tt.wantImport, importPath)
				assert.NotEqual(t, Range{}, importRange)
			} else {
				assert.Equal(t, "", importPath)
			}
		})
	}
}

func TestHoverProvider_GetHover_ViolatedImport(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create a test Go file with a prohibited import
	testFile := `package api

import (
	"internal/database"
)

func GetUser() {
}
`
	err := afero.WriteFile(fs, "internal/api/handler.go", []byte(testFile), 0644)
	require.NoError(t, err)

	// Create go.mod in project root
	goMod := `module github.com/test/app

go 1.24
`
	err = afero.WriteFile(fs, "go.mod", []byte(goMod), 0644)
	require.NoError(t, err)

	config := &goverhaul.Config{
		Rules: []goverhaul.Rule{
			{
				Path: "internal/api",
				Prohibited: []goverhaul.ProhibitedPkg{
					{
						Name:  "internal/database",
						Cause: "API layer must not access database directly",
					},
				},
			},
		},
		Modfile: "go.mod",
	}

	linter, err := goverhaul.NewLinter(*config, slog.New(slog.NewTextHandler(os.Stdout, nil)), fs)
	require.NoError(t, err)

	provider := NewHoverProvider(linter, config, fs)

	// Position on the "internal/database" import (line 3, 0-indexed)
	hover, err := provider.GetHover("file:///internal/api/handler.go", Position{Line: 3, Character: 5})
	require.NoError(t, err)
	require.NotNil(t, hover)

	assert.Equal(t, Markdown, hover.Contents.Kind)
	assert.Contains(t, hover.Contents.Value, "internal/database")
	assert.Contains(t, hover.Contents.Value, "Violations")
	assert.Contains(t, hover.Contents.Value, "API layer must not access database directly")
}

func TestHoverProvider_GetHover_AllowedImport(t *testing.T) {
	fs := afero.NewMemMapFs()

	testFile := `package api

import (
	"context"
)

func GetUser(ctx context.Context) {
}
`
	err := afero.WriteFile(fs, "internal/api/handler.go", []byte(testFile), 0644)
	require.NoError(t, err)

	// Create go.mod in project root
	goMod := `module github.com/test/app

go 1.24
`
	err = afero.WriteFile(fs, "go.mod", []byte(goMod), 0644)
	require.NoError(t, err)

	config := &goverhaul.Config{
		Rules: []goverhaul.Rule{
			{
				Path: "internal/api",
				Allowed: []string{
					"context",
					"fmt",
				},
			},
		},
		Modfile: "go.mod",
	}

	linter, err := goverhaul.NewLinter(*config, slog.New(slog.NewTextHandler(os.Stdout, nil)), fs)
	require.NoError(t, err)

	provider := NewHoverProvider(linter, config, fs)

	hover, err := provider.GetHover("file:///internal/api/handler.go", Position{Line: 3, Character: 5})
	require.NoError(t, err)
	require.NotNil(t, hover)

	assert.Equal(t, Markdown, hover.Contents.Kind)
	assert.Contains(t, hover.Contents.Value, "context")
	assert.Contains(t, hover.Contents.Value, "Allowed")
}

func TestHoverProvider_GetHover_NoRules(t *testing.T) {
	fs := afero.NewMemMapFs()

	testFile := `package main

import (
	"encoding/json"
)

func main() {
}
`
	err := afero.WriteFile(fs, "main.go", []byte(testFile), 0644)
	require.NoError(t, err)

	config := &goverhaul.Config{
		Rules: []goverhaul.Rule{},
	}

	linter, err := goverhaul.NewLinter(*config, slog.New(slog.NewTextHandler(os.Stdout, nil)), fs)
	require.NoError(t, err)

	provider := NewHoverProvider(linter, config, fs)

	hover, err := provider.GetHover("file:///main.go", Position{Line: 3, Character: 5})
	require.NoError(t, err)
	require.NotNil(t, hover)

	assert.Equal(t, Markdown, hover.Contents.Kind)
	assert.Contains(t, hover.Contents.Value, "encoding/json")
	assert.Contains(t, hover.Contents.Value, "No Rules")
}

func TestHoverProvider_GetHover_NotOnImport(t *testing.T) {
	fs := afero.NewMemMapFs()

	testFile := `package main

import (
	"fmt"
)

func main() {
	fmt.Println("hello")
}
`
	err := afero.WriteFile(fs, "main.go", []byte(testFile), 0644)
	require.NoError(t, err)

	config := &goverhaul.Config{
		Rules: []goverhaul.Rule{},
	}

	linter, err := goverhaul.NewLinter(*config, slog.New(slog.NewTextHandler(os.Stdout, nil)), fs)
	require.NoError(t, err)

	provider := NewHoverProvider(linter, config, fs)

	// Position on function main (not on import)
	hover, err := provider.GetHover("file:///main.go", Position{Line: 6, Character: 0})
	require.NoError(t, err)
	assert.Nil(t, hover)
}

func TestURIToPath(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		want    string
		wantErr bool
	}{
		{
			name:    "unix file path",
			uri:     "file:///home/user/project/main.go",
			want:    "/home/user/project/main.go",
			wantErr: false,
		},
		{
			name:    "relative unix path",
			uri:     "file:///./main.go",
			want:    "/main.go",
			wantErr: false,
		},
		{
			name:    "invalid scheme",
			uri:     "http://example.com/file.go",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid uri",
			uri:     "not a uri",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := uriToPath(tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestMatchesImport(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		importPath string
		want       bool
	}{
		{
			name:       "exact match",
			pattern:    "internal/database",
			importPath: "internal/database",
			want:       true,
		},
		{
			name:       "wildcard match",
			pattern:    "internal/*",
			importPath: "internal/database",
			want:       true,
		},
		{
			name:       "recursive wildcard match",
			pattern:    "internal/...",
			importPath: "internal/database/sql",
			want:       true,
		},
		{
			name:       "no match",
			pattern:    "internal/database",
			importPath: "external/api",
			want:       false,
		},
		{
			name:       "wildcard no match",
			pattern:    "internal/*",
			importPath: "external/database",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesImport(tt.pattern, tt.importPath)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetSeverityIcon(t *testing.T) {
	tests := []struct {
		severity string
		want     string
	}{
		{"error", "❌"},
		{"warning", "⚠️"},
		{"info", "ℹ️"},
		{"hint", "💡"},
		{"unknown", "•"},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			got := getSeverityIcon(tt.severity)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildHoverContent(t *testing.T) {
	fs := afero.NewMemMapFs()
	config := &goverhaul.Config{}
	linter, err := goverhaul.NewLinter(*config, slog.New(slog.NewTextHandler(os.Stdout, nil)), fs)
	require.NoError(t, err)

	provider := NewHoverProvider(linter, config, fs)

	tests := []struct {
		name           string
		importPath     string
		violations     []goverhaul.LintViolation
		ruleInfo       *RuleInfo
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:       "with violations",
			importPath: "internal/database",
			violations: []goverhaul.LintViolation{
				{
					File:    "/test.go",
					Import:  "internal/database",
					Rule:    "api-no-db",
					Cause:   "Direct database access not allowed",
					Details: "Use repository pattern",
				},
			},
			ruleInfo: &RuleInfo{
				Path:      "internal/api",
				IsAllowed: false,
			},
			wantContains: []string{
				"Import: `internal/database`",
				"Violations",
				"Direct database access not allowed",
				"Use repository pattern",
			},
		},
		{
			name:       "allowed import",
			importPath: "context",
			violations: []goverhaul.LintViolation{},
			ruleInfo: &RuleInfo{
				Path:      "internal/api",
				IsAllowed: true,
			},
			wantContains: []string{
				"Import: `context`",
				"Allowed",
				"internal/api",
			},
			wantNotContain: []string{
				"Violations",
			},
		},
		{
			name:       "no rules",
			importPath: "encoding/json",
			violations: []goverhaul.LintViolation{},
			ruleInfo:   nil,
			wantContains: []string{
				"Import: `encoding/json`",
				"No Rules",
			},
			wantNotContain: []string{
				"Violations",
				"Allowed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := provider.buildHoverContent(tt.importPath, tt.violations, tt.ruleInfo)

			assert.Equal(t, Markdown, content.Kind)

			for _, want := range tt.wantContains {
				assert.Contains(t, content.Value, want)
			}

			for _, notWant := range tt.wantNotContain {
				assert.NotContains(t, content.Value, notWant)
			}
		})
	}
}
