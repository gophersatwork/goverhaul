package goverhaul

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLinter(t *testing.T) {
	tests := map[string]struct {
		cfg           Config
		setupFs       func(fs afero.Fs) error
		expectError   bool
		errorContains string
	}{
		"should create linter with default config": {
			cfg: Config{
				Modfile:     "go.mod",
				Incremental: false,
				CacheFile:   "cache.json",
			},
			setupFs: func(fs afero.Fs) error {
				return nil
			},
			expectError: false,
		},
		"should create linter with incremental config": {
			cfg: Config{
				Modfile:     "go.mod",
				Incremental: true,
				CacheFile:   "cache.json",
			},
			setupFs: func(fs afero.Fs) error {
				// Create cache directory
				err := fs.MkdirAll(filepath.Dir("cache.json"), 0o755)
				return err
			},
			expectError: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			err := test.setupFs(fs)
			require.NoError(t, err, "Failed to setup filesystem")

			linter, err := NewLinter(test.cfg, nil, fs)

			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.errorContains)
				assert.Nil(t, linter)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, linter)
				assert.Equal(t, test.cfg, linter.cfg)
				assert.NotNil(t, linter.logger)
				assert.Equal(t, fs, linter.fs)
				if test.cfg.Incremental {
					assert.NotNil(t, linter.cache)
				} else {
					assert.Nil(t, linter.cache)
				}
			}
		})
	}
}

func TestEnsureLogger(t *testing.T) {
	t.Run("should return provided logger", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		result := ensureLogger(logger)
		assert.Equal(t, logger, result)
	})

	t.Run("should create default logger when nil", func(t *testing.T) {
		result := ensureLogger(nil)
		assert.NotNil(t, result)
	})
}

func TestLintSuccess(t *testing.T) {
	tests := map[string]struct {
		setupFs func(fs afero.Fs) error
		config  Config
		path    string
	}{
		"should lint directory with no violations": {
			setupFs: func(fs afero.Fs) error {
				// Create a go.mod file
				err := afero.WriteFile(fs, "go.mod", []byte("module example.com\n\ngo 1.20\n"), 0o644)
				if err != nil {
					return err
				}

				// Create a directory with a Go file
				err = fs.MkdirAll("pkg", 0o755)
				if err != nil {
					return err
				}

				// Create a Go file with allowed imports
				return afero.WriteFile(fs, "pkg/main.go", []byte(`package main

import (
	"fmt"
	"errors"
)

func main() {
	fmt.Println("Hello, world!")
}
`), 0o644)
			},
			config: Config{
				Modfile: "go.mod",
				Rules: []Rule{
					{
						Path:    "pkg",
						Allowed: []string{"fmt", "errors"},
					},
				},
			},
			path: "pkg",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			memFs := afero.NewMemMapFs()
			err := test.setupFs(memFs)
			require.NoError(t, err, "Failed to setup filesystem")

			linter, err := NewLinter(test.config, nil, memFs)
			require.NoError(t, err, "Failed to create linter")

			err = linter.Lint(test.path)
			assert.NoError(t, err)
		})
	}
}

func TestLintFailure(t *testing.T) {
	tests := map[string]struct {
		setupFs       func(fs afero.Fs) error
		config        Config
		path          string
		errorContains string
	}{
		"should detect violations": {
			setupFs: func(fs afero.Fs) error {
				// Create a go.mod file
				err := afero.WriteFile(fs, "go.mod", []byte("module example.com\n\ngo 1.20\n"), 0o644)
				if err != nil {
					return err
				}

				// Create a directory with a Go file
				err = fs.MkdirAll("internal", 0o755)
				if err != nil {
					return err
				}

				// Create a Go file with prohibited imports
				return afero.WriteFile(fs, "internal/db.go", []byte(`package db

import (
	"fmt"
	"unsafe"
)

func Connect() {
	fmt.Println("Connecting to database...")
}
`), 0o644)
			},
			config: Config{
				Modfile: "go.mod",
				Rules: []Rule{
					{
						Path: "internal",
						Prohibited: []ProhibitedPkg{
							{
								Name:  "unsafe",
								Cause: "unsafe code is not allowed in internal packages",
							},
						},
					},
				},
			},
			path:          "internal",
			errorContains: "lint errors found",
		},
		"should handle non-existent path": {
			setupFs: func(fs afero.Fs) error {
				return nil
			},
			config: Config{
				Modfile: "go.mod",
			},
			path:          "non-existent",
			errorContains: "file does not exist",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			memFs := afero.NewMemMapFs()
			err := test.setupFs(memFs)
			require.NoError(t, err, "Failed to setup filesystem")

			linter, err := NewLinter(test.config, nil, memFs)
			require.NoError(t, err, "Failed to create linter")

			err = linter.Lint(test.path)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), test.errorContains)
		})
	}
}

func TestWalkAndLintSuccess(t *testing.T) {
	tests := map[string]struct {
		setupFs            func(fs afero.Fs) error
		config             Config
		path               string
		expectedViolations int
	}{
		"should walk and lint directory with no violations": {
			setupFs: func(fs afero.Fs) error {
				// Create a go.mod file
				err := afero.WriteFile(fs, "go.mod", []byte("module example.com\n\ngo 1.20\n"), 0o644)
				if err != nil {
					return err
				}

				// Create a directory structure
				dirs := []string{"pkg", "pkg/subpkg", "cmd"}
				for _, dir := range dirs {
					err = fs.MkdirAll(dir, 0o755)
					if err != nil {
						return err
					}
				}

				// Create Go files with allowed imports
				files := map[string]string{
					"pkg/main.go": `package main
import (
	"fmt"
	"errors"
)
func main() {}`,
					"pkg/subpkg/util.go": `package subpkg
import (
	"fmt"
)
func Util() {}`,
					"cmd/app.go": `package main
import (
	"fmt"
	"os"
)
func main() {}`,
				}

				for path, content := range files {
					err = afero.WriteFile(fs, path, []byte(content), 0o644)
					if err != nil {
						return err
					}
				}

				return nil
			},
			config: Config{
				Modfile: "go.mod",
				Rules: []Rule{
					{
						Path:    "pkg",
						Allowed: []string{"fmt", "errors"},
					},
					{
						Path:    "cmd",
						Allowed: []string{"fmt", "os"},
					},
				},
			},
			path:               "",
			expectedViolations: 0,
		},
		"should detect violations in multiple files": {
			setupFs: func(fs afero.Fs) error {
				// Create a go.mod file
				err := afero.WriteFile(fs, "go.mod", []byte("module example.com\n\ngo 1.20\n"), 0o644)
				if err != nil {
					return err
				}

				// Create a directory structure
				dirs := []string{"internal", "internal/api", "internal/db"}
				for _, dir := range dirs {
					err = fs.MkdirAll(dir, 0o755)
					if err != nil {
						return err
					}
				}

				// Create Go files with some prohibited imports
				files := map[string]string{
					"internal/api/api.go": `package api
import (
	"fmt"
	"unsafe" // Prohibited
)
func API() {}`,
					"internal/db/db.go": `package db
import (
	"fmt"
	"database/sql"
	"unsafe" // Prohibited
)
func DB() {}`,
				}

				for path, content := range files {
					err = afero.WriteFile(fs, path, []byte(content), 0o644)
					if err != nil {
						return err
					}
				}

				return nil
			},
			config: Config{
				Modfile: "go.mod",
				Rules: []Rule{
					{
						Path: "internal",
						Prohibited: []ProhibitedPkg{
							{
								Name:  "unsafe",
								Cause: "unsafe code is not allowed in internal packages",
							},
						},
					},
				},
			},
			path:               "internal",
			expectedViolations: 2, // Two files with unsafe import
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			memFs := afero.NewMemMapFs()
			err := test.setupFs(memFs)
			require.NoError(t, err, "Failed to setup filesystem")

			linter, err := NewLinter(test.config, nil, memFs)
			require.NoError(t, err, "Failed to create linter")

			violations, err := linter.walkAndLint(test.path)
			assert.NoError(t, err)
			assert.NotNil(t, violations)
			assert.Equal(t, test.expectedViolations, len(violations.Violations))
		})
	}
}

func TestWalkAndLintFailure(t *testing.T) {
	tests := map[string]struct {
		setupFs       func(fs afero.Fs) error
		config        Config
		path          string
		errorContains string
	}{
		"should handle permission error": {
			setupFs: func(fs afero.Fs) error {
				// Create a directory that we'll pretend has permission issues
				return fs.MkdirAll("restricted", 0o755)
			},
			config: Config{
				Modfile: "go.mod",
			},
			path:          "restricted",
			errorContains: "error accessing path",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			memFs := afero.NewMemMapFs()
			err := test.setupFs(memFs)
			require.NoError(t, err, "Failed to setup filesystem")

			// Create a mock filesystem that returns permission error
			mockFs := &mockErrorFs{
				Fs:        memFs,
				errorPath: "restricted",
				err:       errors.New("permission denied"),
			}

			linter, err := NewLinter(test.config, nil, mockFs)
			require.NoError(t, err, "Failed to create linter")

			violations, err := linter.walkAndLint(test.path)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), test.errorContains)
			assert.Nil(t, violations)
		})
	}
}

func TestLintFileSuccess(t *testing.T) {
	tests := map[string]struct {
		setupFs            func(fs afero.Fs) error
		config             Config
		filePath           string
		expectedViolations int
	}{
		"should lint file with no violations": {
			setupFs: func(fs afero.Fs) error {
				// Create a go.mod file
				err := afero.WriteFile(fs, "go.mod", []byte("module example.com\n\ngo 1.20\n"), 0o644)
				if err != nil {
					return err
				}

				// Create a directory
				err = fs.MkdirAll("pkg", 0o755)
				if err != nil {
					return err
				}

				// Create a Go file with allowed imports
				return afero.WriteFile(fs, "pkg/main.go", []byte(`package main

import (
	"fmt"
	"errors"
)

func main() {
	fmt.Println("Hello, world!")
}
`), 0o644)
			},
			config: Config{
				Modfile: "go.mod",
				Rules: []Rule{
					{
						Path:    "pkg",
						Allowed: []string{"fmt", "errors"},
					},
				},
			},
			filePath:           "pkg/main.go",
			expectedViolations: 0,
		},
		"should detect violations in file": {
			setupFs: func(fs afero.Fs) error {
				// Create a go.mod file
				err := afero.WriteFile(fs, "go.mod", []byte("module example.com\n\ngo 1.20\n"), 0o644)
				if err != nil {
					return err
				}

				// Create a directory
				err = fs.MkdirAll("internal", 0o755)
				if err != nil {
					return err
				}

				// Create a Go file with prohibited imports
				return afero.WriteFile(fs, "internal/db.go", []byte(`package db

import (
	"fmt"
	"unsafe"
)

func Connect() {
	fmt.Println("Connecting to database...")
}
`), 0o644)
			},
			config: Config{
				Modfile: "go.mod",
				Rules: []Rule{
					{
						Path: "internal",
						Prohibited: []ProhibitedPkg{
							{
								Name:  "unsafe",
								Cause: "unsafe code is not allowed in internal packages",
							},
						},
					},
				},
			},
			filePath:           "internal/db.go",
			expectedViolations: 1,
		},
		"should handle invalid Go file": {
			setupFs: func(fs afero.Fs) error {
				// Create an invalid Go file
				return afero.WriteFile(fs, "invalid.go", []byte(`package main

import (
	"fmt
	"errors"
)

func main() {
	fmt.Println("Hello, world!")
}
`), 0o644)
			},
			config: Config{
				Modfile: "go.mod",
			},
			filePath:           "invalid.go",
			expectedViolations: 0,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			memFs := afero.NewMemMapFs()
			err := test.setupFs(memFs)
			require.NoError(t, err, "Failed to setup filesystem")

			linter, err := NewLinter(test.config, nil, memFs)
			require.NoError(t, err, "Failed to create linter")

			violations := NewLintViolations()
			err = linter.lintFile(test.filePath, violations)
			assert.NoError(t, err)
			assert.Equal(t, test.expectedViolations, len(violations.Violations))
		})
	}
}

func TestLintFileFailure(t *testing.T) {
	// Note: The original TestLintFile didn't have any failure cases
	// where lintFile returns an error. This is a placeholder for future
	// failure tests if they are added.
	t.Skip("No failure cases for lintFile function")
}

func TestGetImportsSuccess(t *testing.T) {
	tests := map[string]struct {
		setupFs         func(fs afero.Fs) error
		filePath        string
		expectedImports []string
	}{
		"should get imports from valid Go file": {
			setupFs: func(fs afero.Fs) error {
				return afero.WriteFile(fs, "main.go", []byte(`package main

import (
	"fmt"
	"errors"
	"os"
)

func main() {
	fmt.Println("Hello, world!")
}
`), 0o644)
			},
			filePath:        "main.go",
			expectedImports: []string{"fmt", "errors", "os"},
		},
		"should handle file with no imports": {
			setupFs: func(fs afero.Fs) error {
				return afero.WriteFile(fs, "empty.go", []byte(`package main

func main() {
	println("Hello, world!")
}
`), 0o644)
			},
			filePath:        "empty.go",
			expectedImports: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			memFs := afero.NewMemMapFs()
			err := test.setupFs(memFs)
			require.NoError(t, err, "Failed to setup filesystem")

			linter, err := NewLinter(Config{}, nil, memFs)
			require.NoError(t, err, "Failed to create linter")

			imports, err := linter.getImports(test.filePath)
			assert.NoError(t, err)
			assert.Equal(t, test.expectedImports, imports)
		})
	}
}

func TestGetImportsFailure(t *testing.T) {
	tests := map[string]struct {
		setupFs       func(fs afero.Fs) error
		filePath      string
		errorContains string
	}{
		"should handle non-existent file": {
			setupFs: func(fs afero.Fs) error {
				return nil
			},
			filePath:      "nonexistent.go",
			errorContains: "failed to read Go file",
		},
		"should handle invalid Go file": {
			setupFs: func(fs afero.Fs) error {
				return afero.WriteFile(fs, "invalid.go", []byte(`package main

import (
	"fmt
	"errors"
)

func main() {
	fmt.Println("Hello, world!")
}
`), 0o644)
			},
			filePath:      "invalid.go",
			errorContains: "failed to parse Go file",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			memFs := afero.NewMemMapFs()
			err := test.setupFs(memFs)
			require.NoError(t, err, "Failed to setup filesystem")

			linter, err := NewLinter(Config{}, nil, memFs)
			require.NoError(t, err, "Failed to create linter")

			imports, err := linter.getImports(test.filePath)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), test.errorContains)
			assert.Nil(t, imports)
		})
	}
}

func TestRuleAppliesToPath(t *testing.T) {
	tests := map[string]struct {
		rule     Rule
		path     string
		expected bool
	}{
		"should match exact path": {
			rule: Rule{
				Path: "internal",
			},
			path:     "internal/file.go",
			expected: true,
		},
		"should match subdirectory": {
			rule: Rule{
				Path: "internal",
			},
			path:     "internal/db/file.go",
			expected: true,
		},
		"should not match different directory": {
			rule: Rule{
				Path: "internal",
			},
			path:     "pkg/file.go",
			expected: false,
		},
		"should handle absolute paths": {
			rule: Rule{
				Path: "/project/internal",
			},
			path:     "/project/internal/file.go",
			expected: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := ruleAppliesToPath(test.rule, test.path)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestRuleMatcher(t *testing.T) {
	tests := map[string]struct {
		rule           Rule
		moduleName     string
		imports        []string
		expectedResult map[string]bool // import -> isViolation
	}{
		"should detect prohibited imports": {
			rule: Rule{
				Path: "internal",
				Prohibited: []ProhibitedPkg{
					{
						Name:  "unsafe",
						Cause: "unsafe code is not allowed",
					},
				},
			},
			moduleName: "example.com",
			imports:    []string{"fmt", "unsafe", "os"},
			expectedResult: map[string]bool{
				"fmt":    false,
				"unsafe": true,
				"os":     false,
			},
		},
		"should detect non-allowed imports": {
			rule: Rule{
				Path:    "pkg",
				Allowed: []string{"fmt", "errors"},
			},
			moduleName: "example.com",
			imports:    []string{"fmt", "errors", "os"},
			expectedResult: map[string]bool{
				"fmt":    false,
				"errors": false,
				"os":     true,
			},
		},
		"should handle module-relative paths": {
			rule: Rule{
				Path: "internal",
				Prohibited: []ProhibitedPkg{
					{
						Name:  "example.com/internal/private",
						Cause: "private packages should not be imported directly",
					},
				},
			},
			moduleName: "example.com",
			imports:    []string{"fmt", "example.com/internal/private", "os"},
			expectedResult: map[string]bool{
				"fmt":                          false,
				"example.com/internal/private": true,
				"os":                           false,
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			memFs := afero.NewMemMapFs()
			matcher := newRuleMatcherWithFs(test.rule, test.moduleName, memFs)

			for _, imp := range test.imports {
				violation := matcher.CheckImport(imp, "test.go", slog.Default())
				expectedViolation := test.expectedResult[imp]

				if expectedViolation {
					assert.NotNil(t, violation, "Expected violation for import %s", imp)
					assert.Equal(t, imp, violation.Import)
				} else {
					assert.Nil(t, violation, "Unexpected violation for import %s", imp)
				}
			}
		})
	}
}

// mockErrorFs is a mock filesystem that returns errors for specific paths
type mockErrorFs struct {
	afero.Fs
	errorPath string
	err       error
}

func (m *mockErrorFs) Stat(name string) (os.FileInfo, error) {
	if name == m.errorPath {
		return nil, m.err
	}
	return m.Fs.Stat(name)
}

func (m *mockErrorFs) Open(name string) (afero.File, error) {
	if name == m.errorPath {
		return nil, m.err
	}
	return m.Fs.Open(name)
}

func (m *mockErrorFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if name == m.errorPath {
		return nil, m.err
	}
	return m.Fs.OpenFile(name, flag, perm)
}

func (m *mockErrorFs) MkdirAll(path string, perm os.FileMode) error {
	if path == filepath.Dir(m.errorPath) {
		return m.err
	}
	return m.Fs.MkdirAll(path, perm)
}
