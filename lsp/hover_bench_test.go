package lsp

import (
	"log/slog"
	"os"
	"testing"

	"github.com/gophersatwork/goverhaul"
	"github.com/spf13/afero"
)

func BenchmarkHoverProvider_GetHover(b *testing.B) {
	fs := afero.NewMemMapFs()

	// Create test files
	testFile := `package api

import (
	"internal/database"
	"context"
	"fmt"
)

func GetUser() {
}
`
	afero.WriteFile(fs, "internal/api/handler.go", []byte(testFile), 0644)

	goMod := `module github.com/test/app

go 1.24
`
	afero.WriteFile(fs, "go.mod", []byte(goMod), 0644)

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
				Allowed: []string{
					"context",
					"fmt",
				},
			},
		},
		Modfile: "go.mod",
	}

	linter, _ := goverhaul.NewLinter(*config, slog.New(slog.NewTextHandler(os.Stdout, nil)), fs)
	provider := NewHoverProvider(linter, config, fs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.GetHover("file:///internal/api/handler.go", Position{Line: 3, Character: 5})
	}
}

func BenchmarkHoverProvider_FindImportAtPosition(b *testing.B) {
	fs := afero.NewMemMapFs()

	testFile := `package api

import (
	"internal/database"
	"context"
	"fmt"
)

func GetUser() {
}
`
	afero.WriteFile(fs, "internal/api/handler.go", []byte(testFile), 0644)

	config := &goverhaul.Config{
		Rules: []goverhaul.Rule{},
	}

	linter, _ := goverhaul.NewLinter(*config, slog.New(slog.NewTextHandler(os.Stdout, nil)), fs)
	provider := NewHoverProvider(linter, config, fs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.findImportAtPosition("file:///internal/api/handler.go", Position{Line: 3, Character: 5})
	}
}

func BenchmarkHoverProvider_BuildHoverContent(b *testing.B) {
	fs := afero.NewMemMapFs()
	config := &goverhaul.Config{}
	linter, _ := goverhaul.NewLinter(*config, slog.New(slog.NewTextHandler(os.Stdout, nil)), fs)
	provider := NewHoverProvider(linter, config, fs)

	violations := []goverhaul.LintViolation{
		{
			File:    "internal/api/handler.go",
			Import:  "internal/database",
			Rule:    "internal/api",
			Cause:   "API layer must not access database directly",
			Details: "Use repository pattern instead",
		},
	}

	ruleInfo := &RuleInfo{
		Path:      "internal/api",
		IsAllowed: false,
		Reason:    "API layer must not access database directly",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.buildHoverContent("internal/database", violations, ruleInfo)
	}
}
