package main

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/afero"
)

func TestGetModuleName(t *testing.T) {
	// Create a temporary file for testing
	tempDir := t.TempDir()
	modfilePath := filepath.Join(tempDir, "go.mod")

	// Test case 1: Valid go.mod file
	validModContent := `module github.com/example/goverhaul

go 1.20

require (
	github.com/spf13/afero v1.9.5
	gopkg.in/yaml.v3 v3.0.1
)
`
	err := os.WriteFile(modfilePath, []byte(validModContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test go.mod file: %v", err)
	}

	t.Run("Valid go.mod file", func(t *testing.T) {
		moduleName, err := getModuleName(modfilePath)
		if err != nil {
			t.Fatalf("getModuleName returned an error: %v", err)
		}

		expectedModuleName := "github.com/example/goverhaul"
		if moduleName != expectedModuleName {
			t.Errorf("Expected module name '%s', got '%s'", expectedModuleName, moduleName)
		}
	})

	// Test case 2: Default path (empty string)
	t.Run("Default path", func(t *testing.T) {
		// Save current directory
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}

		// Change to temp directory
		err = os.Chdir(tempDir)
		if err != nil {
			t.Fatalf("Failed to change directory: %v", err)
		}
		defer os.Chdir(currentDir) // Restore original directory

		moduleName, err := getModuleName("")
		if err != nil {
			t.Fatalf("getModuleName returned an error: %v", err)
		}

		expectedModuleName := "github.com/example/goverhaul"
		if moduleName != expectedModuleName {
			t.Errorf("Expected module name '%s', got '%s'", expectedModuleName, moduleName)
		}
	})

	// Test case 3: Non-existent file
	t.Run("Non-existent file", func(t *testing.T) {
		_, err := getModuleName(filepath.Join(tempDir, "nonexistent.mod"))
		if err == nil {
			t.Error("Expected an error for non-existent file, got nil")
		}
	})
}

func TestGetImports(t *testing.T) {
	// Create a temporary file for testing
	tempDir := t.TempDir()

	// Test case 1: Valid Go file with imports
	validGoContent := `package test

import (
	"fmt"
	"errors"
	"github.com/example/package"
)

func TestFunc() {
	fmt.Println("test")
}
`
	validGoPath := filepath.Join(tempDir, "valid.go")
	err := os.WriteFile(validGoPath, []byte(validGoContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test Go file: %v", err)
	}

	t.Run("Valid Go file with imports", func(t *testing.T) {
		imports, err := getImports(validGoPath)
		if err != nil {
			t.Fatalf("getImports returned an error: %v", err)
		}

		expectedImports := []string{"fmt", "errors", "github.com/example/package"}
		if !reflect.DeepEqual(imports, expectedImports) {
			t.Errorf("Expected imports %v, got %v", expectedImports, imports)
		}
	})

	// Test case 2: Go file without imports
	noImportsContent := `package test

func TestFunc() {
	println("test")
}
`
	noImportsPath := filepath.Join(tempDir, "noimports.go")
	err = os.WriteFile(noImportsPath, []byte(noImportsContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test Go file: %v", err)
	}

	t.Run("Go file without imports", func(t *testing.T) {
		imports, err := getImports(noImportsPath)
		if err != nil {
			t.Fatalf("getImports returned an error: %v", err)
		}

		if len(imports) != 0 {
			t.Errorf("Expected empty imports, got %v", imports)
		}
	})

	// Test case 3: Invalid Go file
	invalidGoContent := `package test

import (
	"fmt
	"errors"
)

func TestFunc() {
	fmt.Println("test")
}
`
	invalidGoPath := filepath.Join(tempDir, "invalid.go")
	err = os.WriteFile(invalidGoPath, []byte(invalidGoContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write invalid Go file: %v", err)
	}

	t.Run("Invalid Go file", func(t *testing.T) {
		_, err := getImports(invalidGoPath)
		if err == nil {
			t.Error("Expected an error for invalid Go file, got nil")
		}
	})
}

func TestCheckImports(t *testing.T) {
	// Create a test logger that discards output
	testLogger := slog.New(slog.NewTextHandler(os.NewFile(0, os.DevNull), nil))

	// Test case 1: Prohibited imports
	t.Run("Prohibited imports", func(t *testing.T) {
		rule := Rule{
			Path: "internal",
			Prohibited: []ProhibitedPkg{
				{Name: "unsafe", Cause: "unsafe code is not allowed"},
				{Name: "github.com/example/private", Cause: "private packages should not be used"},
			},
		}
		// For imports without dots, the function adds the module name prefix
		// So we need to use the full path for "unsafe"
		imports := []string{"fmt", "github.com/example/goverhaul/unsafe", "github.com/example/public"}
		moduleName := "github.com/example/goverhaul"

		violations := checkImports("internal/file.go", imports, rule, moduleName, testLogger)

		if len(violations) == 0 {
			t.Error("Expected violations, got none")
		}
	})

	// Test case 2: Allowed imports
	t.Run("Allowed imports", func(t *testing.T) {
		rule := Rule{
			Path:    "internal",
			Allowed: []string{"fmt", "errors"},
		}
		imports := []string{"fmt", "errors"}
		moduleName := "github.com/example/goverhaul"

		violations := checkImports("internal/file.go", imports, rule, moduleName, testLogger)

		if len(violations) > 0 {
			t.Errorf("Expected no violations, got %d", len(violations))
		}
	})

	// Test case 3: Not allowed imports
	t.Run("Not allowed imports", func(t *testing.T) {
		rule := Rule{
			Path:    "internal",
			Allowed: []string{"fmt", "errors"},
		}
		imports := []string{"fmt", "unsafe"}
		moduleName := "github.com/example/goverhaul"

		violations := checkImports("internal/file.go", imports, rule, moduleName, testLogger)

		if len(violations) == 0 {
			t.Error("Expected violations, got none")
		}
	})

	// Test case 4: Module-relative imports
	t.Run("Module-relative imports", func(t *testing.T) {
		rule := Rule{
			Path:    "cmd",
			Allowed: []string{"public"},
			Prohibited: []ProhibitedPkg{
				{Name: "private", Cause: "private packages should not be used directly"},
			},
		}
		imports := []string{"github.com/example/goverhaul/public", "github.com/example/goverhaul/private"}
		moduleName := "github.com/example/goverhaul"

		violations := checkImports("cmd/file.go", imports, rule, moduleName, testLogger)

		if len(violations) == 0 {
			t.Error("Expected violations, got none")
		}
	})
}

func TestLint(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a logger that discards output for testing
	testLogger := slog.New(slog.NewTextHandler(os.NewFile(0, os.DevNull), nil))

	// Test case 1: Basic linting with no violations
	t.Run("Basic linting with no violations", func(t *testing.T) {
		// Create test directory structure
		projectDir := filepath.Join(tempDir, "project1")
		internalDir := filepath.Join(projectDir, "internal")
		apiDir := filepath.Join(internalDir, "api")
		domainDir := filepath.Join(internalDir, "domain")

		// Create directories
		for _, dir := range []string{projectDir, internalDir, apiDir, domainDir} {
			err := os.MkdirAll(dir, 0o755)
			if err != nil {
				t.Fatalf("Failed to create directory %s: %v", dir, err)
			}
		}

		// Create go.mod file
		modContent := `module github.com/example/goverhaul

go 1.20
`
		err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(modContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write go.mod file: %v", err)
		}

		// Create config file
		configContent := `rules:
  - path: "internal/domain"
    allowed:
      - "fmt"
      - "errors"
  - path: "internal/api"
    allowed:
      - "fmt"
      - "errors"
      - "internal/domain"
`
		err = os.WriteFile(filepath.Join(projectDir, ".goverhaul.yml"), []byte(configContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		// Create valid Go files
		apiGoContent := `package api

import (
	"fmt"
	"errors"
	"github.com/example/goverhaul/internal/domain"
)

func APIFunc() {
	fmt.Println("api")
	domain.DomainFunc()
}
`
		err = os.WriteFile(filepath.Join(apiDir, "api.go"), []byte(apiGoContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write api.go file: %v", err)
		}

		domainGoContent := `package domain

import (
	"fmt"
	"errors"
)

func DomainFunc() {
	fmt.Println("domain")
}
`
		err = os.WriteFile(filepath.Join(domainDir, "domain.go"), []byte(domainGoContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write domain.go file: %v", err)
		}

		// Save current directory
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}

		// Change to project directory
		err = os.Chdir(projectDir)
		if err != nil {
			t.Fatalf("Failed to change directory: %v", err)
		}
		defer os.Chdir(currentDir) // Restore original directory

		// Load config
		cfg, err := LoadConfig(afero.NewOsFs(), ".goverhaul.yml")
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Run linter
		err = Lint(".", cfg, testLogger)
		if err != nil {
			t.Errorf("Expected no lint errors, got: %v", err)
		}
	})

	// Test case 2: Linting with violations
	t.Run("Linting with violations", func(t *testing.T) {
		// Create test directory structure
		projectDir := filepath.Join(tempDir, "project2")
		internalDir := filepath.Join(projectDir, "internal")
		apiDir := filepath.Join(internalDir, "api")
		domainDir := filepath.Join(internalDir, "domain")
		infraDir := filepath.Join(internalDir, "infrastructure")

		// Create directories
		for _, dir := range []string{projectDir, internalDir, apiDir, domainDir, infraDir} {
			err := os.MkdirAll(dir, 0o755)
			if err != nil {
				t.Fatalf("Failed to create directory %s: %v", dir, err)
			}
		}

		// Create go.mod file
		modContent := `module github.com/example/goverhaul

go 1.20
`
		err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(modContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write go.mod file: %v", err)
		}

		// Create config file
		configContent := `rules:
  - path: "internal/domain"
    prohibited:
      - name: "internal/infrastructure"
        cause: "Domain should not depend on infrastructure"
`
		err = os.WriteFile(filepath.Join(projectDir, ".goverhaul.yml"), []byte(configContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		// Create Go file with prohibited import
		domainGoContent := `package domain

import (
	"fmt"
	"github.com/example/goverhaul/internal/infrastructure"
)

func DomainFunc() {
	fmt.Println("domain")
	infrastructure.InfraFunc()
}
`
		err = os.WriteFile(filepath.Join(domainDir, "domain.go"), []byte(domainGoContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write domain.go file: %v", err)
		}

		// Create infrastructure file
		infraGoContent := `package infrastructure

func InfraFunc() {
	// Infrastructure code
}
`
		err = os.WriteFile(filepath.Join(infraDir, "infra.go"), []byte(infraGoContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write infra.go file: %v", err)
		}

		// Save current directory
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}

		// Change to project directory
		err = os.Chdir(projectDir)
		if err != nil {
			t.Fatalf("Failed to change directory: %v", err)
		}
		defer os.Chdir(currentDir) // Restore original directory

		// Load config
		cfg, err := LoadConfig(afero.NewOsFs(), ".goverhaul.yml")
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Run linter
		err = Lint(".", cfg, testLogger)
		if err == nil {
			t.Error("Expected lint errors, got nil")
		} else if !errors.Is(err, ErrLint) {
			t.Errorf("Expected ErrLint, got: %v", err)
		}
	})

	// Test case 3: Running from subdirectory
	t.Run("Running from subdirectory", func(t *testing.T) {
		// Create test directory structure
		projectDir := filepath.Join(tempDir, "project3")
		internalDir := filepath.Join(projectDir, "internal")
		apiDir := filepath.Join(internalDir, "api")
		dbDir := filepath.Join(internalDir, "database")

		// Create directories
		for _, dir := range []string{projectDir, internalDir, apiDir, dbDir} {
			err := os.MkdirAll(dir, 0o755)
			if err != nil {
				t.Fatalf("Failed to create directory %s: %v", dir, err)
			}
		}

		// Create go.mod file
		modContent := `module github.com/example/goverhaul

go 1.20
`
		err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(modContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write go.mod file: %v", err)
		}

		// Create config file
		configContent := `rules:
  - path: "internal/api"
    prohibited:
      - name: "internal/database"
        cause: "API should not access database directly"
modfile: "../go.mod"
`
		err = os.WriteFile(filepath.Join(projectDir, ".goverhaul.yml"), []byte(configContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		// Create API file with prohibited import
		apiGoContent := `package api

import (
	"fmt"
	"github.com/example/goverhaul/internal/database"
)

func APIFunc() {
	fmt.Println("api")
	database.QueryDB()
}
`
		err = os.WriteFile(filepath.Join(apiDir, "api.go"), []byte(apiGoContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write api.go file: %v", err)
		}

		// Create database file
		dbGoContent := `package database

func QueryDB() {
	// Database code
}
`
		err = os.WriteFile(filepath.Join(dbDir, "db.go"), []byte(dbGoContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write db.go file: %v", err)
		}

		// Save current directory
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}

		// Change to internal directory (subdirectory)
		err = os.Chdir(internalDir)
		if err != nil {
			t.Fatalf("Failed to change directory: %v", err)
		}
		defer os.Chdir(currentDir) // Restore original directory

		// Load config from parent directory
		cfg, err := LoadConfig(afero.NewOsFs(), "../.goverhaul.yml")
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Run linter from subdirectory
		err = Lint(".", cfg, testLogger)
		if err == nil {
			t.Error("Expected lint errors, got nil")
		} else if err != ErrLint {
			t.Errorf("Expected ErrLint, got: %v", err)
		}
	})

	// Test case 4: Absolute paths
	t.Run("Absolute paths", func(t *testing.T) {
		// Create test directory structure
		projectDir := filepath.Join(tempDir, "project4")
		internalDir := filepath.Join(projectDir, "internal")
		apiDir := filepath.Join(internalDir, "api")
		domainDir := filepath.Join(internalDir, "domain")

		// Create directories
		for _, dir := range []string{projectDir, internalDir, apiDir, domainDir} {
			err := os.MkdirAll(dir, 0o755)
			if err != nil {
				t.Fatalf("Failed to create directory %s: %v", dir, err)
			}
		}

		// Create go.mod file
		modContent := `module github.com/example/goverhaul

go 1.20
`
		err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(modContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write go.mod file: %v", err)
		}

		// Create config file with absolute paths
		configContent := `rules:
  - path: "` + filepath.ToSlash(apiDir) + `"
    allowed:
      - "fmt"
      - "errors"
`
		err = os.WriteFile(filepath.Join(projectDir, ".goverhaul.yml"), []byte(configContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		// Create API file with allowed imports
		apiGoContent := `package api

import (
	"fmt"
	"errors"
)

func APIFunc() {
	fmt.Println("api")
}
`
		err = os.WriteFile(filepath.Join(apiDir, "api.go"), []byte(apiGoContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write api.go file: %v", err)
		}

		// Save current directory
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}

		// Change to project directory
		err = os.Chdir(projectDir)
		if err != nil {
			t.Fatalf("Failed to change directory: %v", err)
		}
		defer os.Chdir(currentDir) // Restore original directory

		// Load config
		cfg, err := LoadConfig(afero.NewOsFs(), ".goverhaul.yml")
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Run linter
		err = Lint(".", cfg, testLogger)
		if err != nil {
			t.Errorf("Expected no lint errors, got: %v", err)
		}
	})

	// Test case 5: Empty path
	t.Run("Empty path", func(t *testing.T) {
		// Create test directory structure
		projectDir := filepath.Join(tempDir, "project5")

		// Create directory
		err := os.MkdirAll(projectDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", projectDir, err)
		}

		// Create go.mod file
		modContent := `module github.com/example/goverhaul

go 1.20
`
		err = os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(modContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write go.mod file: %v", err)
		}

		// Create config file with empty path
		configContent := `rules:
  - path: ""
    allowed:
      - "fmt"
`
		err = os.WriteFile(filepath.Join(projectDir, ".goverhaul.yml"), []byte(configContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		// Create Go file
		goContent := `package main

import (
	"fmt"
)

func main() {
	fmt.Println("Hello, world!")
}
`
		err = os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(goContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write main.go file: %v", err)
		}

		// Save current directory
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}

		// Change to project directory
		err = os.Chdir(projectDir)
		if err != nil {
			t.Fatalf("Failed to change directory: %v", err)
		}
		defer os.Chdir(currentDir) // Restore original directory

		// Load config
		cfg, err := LoadConfig(afero.NewOsFs(), ".goverhaul.yml")
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Run linter
		err = Lint(".", cfg, testLogger)
		if err != nil {
			t.Errorf("Expected no lint errors, got: %v", err)
		}
	})
}
