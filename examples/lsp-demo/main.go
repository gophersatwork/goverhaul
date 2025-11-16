package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/gophersatwork/goverhaul"
	"github.com/gophersatwork/goverhaul/lsp"
	"github.com/spf13/afero"
)

func main() {
	fmt.Println("=== Goverhaul LSP Hover Provider Demo ===")
	fmt.Println()

	// Create an in-memory file system for the demo
	fs := afero.NewMemMapFs()

	// Create a sample project structure
	setupSampleProject(fs)

	// Load configuration
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
					"internal/service",
				},
			},
		},
		Modfile: "go.mod",
	}

	// Create linter
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	linter, err := goverhaul.NewLinter(*config, logger, fs)
	if err != nil {
		log.Fatal(err)
	}

	// Create hover provider
	provider := lsp.NewHoverProvider(linter, config, fs)

	// Demo 1: Hover on a violated import
	fmt.Println("Demo 1: Hover on a prohibited import (internal/database)")
	fmt.Println("-------------------------------------------------------")
	hover, err := provider.GetHover(
		"file:///internal/api/handler.go",
		lsp.Position{Line: 3, Character: 5}, // Position on "internal/database"
	)
	if err != nil {
		log.Fatal(err)
	}
	if hover != nil {
		fmt.Printf("Hover Content:\n%s\n\n", hover.Contents.Value)
	}

	// Demo 2: Hover on an allowed import
	fmt.Println("Demo 2: Hover on an allowed import (context)")
	fmt.Println("--------------------------------------------")
	hover, err = provider.GetHover(
		"file:///internal/api/service.go",
		lsp.Position{Line: 3, Character: 5}, // Position on "context"
	)
	if err != nil {
		log.Fatal(err)
	}
	if hover != nil {
		fmt.Printf("Hover Content:\n%s\n\n", hover.Contents.Value)
	}

	// Demo 3: Hover on unconfigured import
	fmt.Println("Demo 3: Hover on an import with no rules (encoding/json)")
	fmt.Println("--------------------------------------------------------")
	hover, err = provider.GetHover(
		"file:///main.go",
		lsp.Position{Line: 3, Character: 5}, // Position on "encoding/json"
	)
	if err != nil {
		log.Fatal(err)
	}
	if hover != nil {
		fmt.Printf("Hover Content:\n%s\n\n", hover.Contents.Value)
	}

	fmt.Println("=== Demo Complete ===")
	fmt.Println("\nNote: This demonstrates how hover tooltips would appear in your editor.")
	fmt.Println("In a real LSP integration, these would show up when you hover over imports.")
}

func setupSampleProject(fs afero.Fs) {
	// Create go.mod
	goMod := `module github.com/example/project

go 1.24
`
	afero.WriteFile(fs, "go.mod", []byte(goMod), 0644)

	// Create a file with a prohibited import
	apiHandler := `package api

import (
	"internal/database"
)

func GetUser() {
	// This violates the architecture rule
}
`
	afero.WriteFile(fs, "internal/api/handler.go", []byte(apiHandler), 0644)

	// Create a file with an allowed import
	apiService := `package api

import (
	"context"
)

func GetUserService(ctx context.Context) {
	// This is allowed
}
`
	afero.WriteFile(fs, "internal/api/service.go", []byte(apiService), 0644)

	// Create a file with no rules
	mainFile := `package main

import (
	"encoding/json"
)

func main() {
	// No rules apply here
}
`
	afero.WriteFile(fs, "main.go", []byte(mainFile), 0644)
}
