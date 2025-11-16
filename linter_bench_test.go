package goverhaul

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
)

// setupBenchmarkFs creates an in-memory filesystem with the specified number of Go files
func setupBenchmarkFs(numFiles int) afero.Fs {
	fs := afero.NewMemMapFs()

	// Create a go.mod file
	modContent := `module benchmark.test

go 1.21
`
	_ = afero.WriteFile(fs, "go.mod", []byte(modContent), 0644)

	// Create Go files with various import patterns
	for i := 0; i < numFiles; i++ {
		dir := fmt.Sprintf("pkg/module%d", i/10)
		filename := filepath.Join(dir, fmt.Sprintf("file%d.go", i))

		content := fmt.Sprintf(`package module%d

import (
	"fmt"
	"context"
	"errors"
	"internal/database"
	"internal/domain"
	"github.com/example/lib"
)

func Function%d() {
	fmt.Println("Hello from function %d")
}
`, i/10, i, i)

		_ = fs.MkdirAll(dir, 0755)
		_ = afero.WriteFile(fs, filename, []byte(content), 0644)
	}

	return fs
}

// setupBenchmarkConfig creates a config with multiple rules
func setupBenchmarkConfig() Config {
	return Config{
		Modfile: "go.mod",
		Rules: []Rule{
			{
				Path: "pkg",
				Allowed: []string{
					"fmt",
					"context",
					"errors",
				},
				Prohibited: []ProhibitedPkg{
					{Name: "internal/database", Cause: "Database should not be imported directly"},
				},
			},
			{
				Path: "internal/domain",
				Allowed: []string{
					"fmt",
					"errors",
				},
				Prohibited: []ProhibitedPkg{
					{Name: "internal/database", Cause: "Domain should not depend on infrastructure"},
				},
			},
		},
	}
}

// BenchmarkLintSingleFile benchmarks linting a single file
func BenchmarkLintSingleFile(b *testing.B) {
	fs := setupBenchmarkFs(1)
	cfg := setupBenchmarkConfig()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	linter, err := NewLinter(cfg, logger, fs)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := linter.Lint(".")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLint10Files benchmarks linting 10 files
func BenchmarkLint10Files(b *testing.B) {
	fs := setupBenchmarkFs(10)
	cfg := setupBenchmarkConfig()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	linter, err := NewLinter(cfg, logger, fs)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := linter.Lint(".")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLint100Files benchmarks linting 100 files
func BenchmarkLint100Files(b *testing.B) {
	fs := setupBenchmarkFs(100)
	cfg := setupBenchmarkConfig()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	linter, err := NewLinter(cfg, logger, fs)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := linter.Lint(".")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLint1000Files benchmarks linting 1000 files
func BenchmarkLint1000Files(b *testing.B) {
	fs := setupBenchmarkFs(1000)
	cfg := setupBenchmarkConfig()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	linter, err := NewLinter(cfg, logger, fs)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := linter.Lint(".")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRuleMatcherIsProhibited benchmarks the IsProhibited method
func BenchmarkRuleMatcherIsProhibited(b *testing.B) {
	rule := Rule{
		Path: "test",
		Prohibited: []ProhibitedPkg{
			{Name: "unsafe", Cause: "not allowed"},
			{Name: "internal/db", Cause: "not allowed"},
			{Name: "internal/cache", Cause: "not allowed"},
			{Name: "internal/auth", Cause: "not allowed"},
			{Name: "internal/api", Cause: "not allowed"},
			{Name: "github.com/example/badlib", Cause: "not allowed"},
			{Name: "github.com/example/deprecated", Cause: "not allowed"},
		},
	}

	matcher := newRuleMatcherWithFs(rule, "example.com/proj", afero.NewMemMapFs())

	testImports := []string{
		"internal/db",
		"fmt",
		"context",
		"github.com/example/goodlib",
		"internal/domain",
		"unsafe",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, imp := range testImports {
			matcher.IsProhibited(imp)
		}
	}
}

// BenchmarkRuleMatcherIsAllowed benchmarks the IsAllowed method
func BenchmarkRuleMatcherIsAllowed(b *testing.B) {
	rule := Rule{
		Path: "test",
		Allowed: []string{
			"fmt",
			"context",
			"errors",
			"time",
			"strings",
			"strconv",
			"internal/domain",
			"internal/core",
			"github.com/example/lib",
		},
	}

	matcher := newRuleMatcherWithFs(rule, "example.com/proj", afero.NewMemMapFs())

	testImports := []string{
		"fmt",
		"internal/db",
		"context",
		"internal/domain",
		"github.com/other/lib",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, imp := range testImports {
			matcher.IsAllowed(imp)
		}
	}
}

// BenchmarkCacheSerializationJSON benchmarks JSON serialization of violations
func BenchmarkCacheSerializationJSON(b *testing.B) {
	violations := make([]LintViolation, 100)
	for i := range violations {
		violations[i] = LintViolation{
			File:   fmt.Sprintf("test/file%d.go", i),
			Import: fmt.Sprintf("internal/package%d", i),
			Rule:   "test-rule",
			Cause:  "Test violation cause",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := json.Marshal(violations)
		if err != nil {
			b.Fatal(err)
		}

		var decoded []LintViolation
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.SetBytes(int64(len(violations)))
}

// BenchmarkCacheSerializationGob benchmarks Gob serialization of violations
func BenchmarkCacheSerializationGob(b *testing.B) {
	violations := make([]LintViolation, 100)
	for i := range violations {
		violations[i] = LintViolation{
			File:   fmt.Sprintf("test/file%d.go", i),
			Import: fmt.Sprintf("internal/package%d", i),
			Rule:   "test-rule",
			Cause:  "Test violation cause",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		err := enc.Encode(violations)
		if err != nil {
			b.Fatal(err)
		}

		dec := gob.NewDecoder(&buf)
		var decoded []LintViolation
		err = dec.Decode(&decoded)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.SetBytes(int64(len(violations)))
}

// BenchmarkGetImports benchmarks the getImports function parsing
func BenchmarkGetImports(b *testing.B) {
	fs := afero.NewMemMapFs()

	content := `package test

import (
	"fmt"
	"context"
	"errors"
	"time"
	"strings"
	"strconv"
	"io"
	"os"
	"path/filepath"
	"net/http"
	"encoding/json"
	"database/sql"
	"internal/domain"
	"internal/repository"
	"github.com/example/lib1"
	"github.com/example/lib2"
	"github.com/example/lib3"
)

func main() {
	fmt.Println("Test")
}
`

	_ = afero.WriteFile(fs, "test.go", []byte(content), 0644)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	linter := &Goverhaul{fs: fs, logger: logger}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := linter.getImports("test.go")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWalkAndLint benchmarks the entire walk and lint process
func BenchmarkWalkAndLint(b *testing.B) {
	testCases := []struct {
		name     string
		numFiles int
	}{
		{"10_files", 10},
		{"50_files", 50},
		{"100_files", 100},
		{"500_files", 500},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			fs := setupBenchmarkFs(tc.numFiles)
			cfg := setupBenchmarkConfig()
			logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

			linter, err := NewLinter(cfg, logger, fs)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := linter.walkAndLint(".")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkViolationsGrouping benchmarks the grouping operations
func BenchmarkViolationsGrouping(b *testing.B) {
	violations := NewLintViolations()

	// Add many violations
	for i := 0; i < 1000; i++ {
		violations.Add(LintViolation{
			File:   fmt.Sprintf("file%d.go", i%100),
			Import: fmt.Sprintf("import%d", i%50),
			Rule:   fmt.Sprintf("rule%d", i%10),
			Cause:  "violation cause",
		})
	}

	b.Run("ByFile", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = violations.PrintByFile()
		}
	})

	b.Run("ByRule", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = violations.PrintByRule()
		}
	})
}