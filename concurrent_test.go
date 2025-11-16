package goverhaul

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProgressReporter is a test implementation of ProgressReporter
type TestProgressReporter struct {
	mu             sync.Mutex
	filesStarted   []string
	filesCompleted []string
	progressUpdates []struct {
		current int
		total   int
	}
	stats *LintStats
}

func (t *TestProgressReporter) StartFile(path string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.filesStarted = append(t.filesStarted, path)
}

func (t *TestProgressReporter) CompleteFile(path string, violations int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.filesCompleted = append(t.filesCompleted, path)
}

func (t *TestProgressReporter) UpdateProgress(current, total int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.progressUpdates = append(t.progressUpdates, struct {
		current int
		total   int
	}{current, total})
}

func (t *TestProgressReporter) Complete(stats *LintStats) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stats = stats
}

func TestConcurrentLinterBasic(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create test files
	setupTestProject(t, fs)

	cfg := Config{
		Modfile: "go.mod",
		Rules: []Rule{
			{
				Path: "internal/api",
				Prohibited: []ProhibitedPkg{
					{Name: "internal/database", Cause: "API should not depend on database directly"},
				},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Create concurrent linter
	linter, err := NewConcurrentLinter(cfg, logger, fs, WithWorkerCount(4))
	require.NoError(t, err)

	// Run linting
	violations, err := linter.LintWithContext(context.Background(), ".")
	require.NoError(t, err)

	// Should find violations
	assert.Greater(t, len(violations.Violations), 0)
}

func TestConcurrentLinterWithContext(t *testing.T) {
	fs := afero.NewMemMapFs()
	setupLargeProject(t, fs, 100) // Create 100 files

	cfg := Config{
		Modfile: "go.mod",
		Rules: []Rule{
			{
				Path: "pkg",
				Prohibited: []ProhibitedPkg{
					{Name: "internal/database", Cause: "Package should not depend on database"},
				},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	t.Run("context cancellation", func(t *testing.T) {
		linter, err := NewConcurrentLinter(cfg, logger, fs)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())

		// Cancel context immediately
		cancel()

		// Should return context error
		_, err = linter.LintWithContext(ctx, ".")
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("context timeout", func(t *testing.T) {
		linter, err := NewConcurrentLinter(cfg, logger, fs)
		require.NoError(t, err)

		// Set a very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond) // Ensure timeout

		// Should return deadline exceeded
		_, err = linter.LintWithContext(ctx, ".")
		assert.Error(t, err)
	})
}

func TestConcurrentLinterOptions(t *testing.T) {
	fs := afero.NewMemMapFs()
	setupTestProject(t, fs)

	cfg := Config{Modfile: "go.mod"}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	t.Run("valid worker count", func(t *testing.T) {
		linter, err := NewConcurrentLinter(cfg, logger, fs, WithWorkerCount(8))
		require.NoError(t, err)
		assert.Equal(t, 8, linter.workerCount)
	})

	t.Run("invalid worker count", func(t *testing.T) {
		_, err := NewConcurrentLinter(cfg, logger, fs, WithWorkerCount(0))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "worker count must be at least 1")
	})

	t.Run("valid buffer size", func(t *testing.T) {
		linter, err := NewConcurrentLinter(cfg, logger, fs, WithBufferSize(200))
		require.NoError(t, err)
		assert.Equal(t, 200, linter.bufferSize)
	})

	t.Run("invalid buffer size", func(t *testing.T) {
		_, err := NewConcurrentLinter(cfg, logger, fs, WithBufferSize(0))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "buffer size must be at least 1")
	})
}

func TestConcurrentLinterProgress(t *testing.T) {
	fs := afero.NewMemMapFs()
	setupLargeProject(t, fs, 20)

	cfg := Config{
		Modfile: "go.mod",
		Rules: []Rule{
			{
				Path: "pkg",
				Allowed: []string{"fmt", "context"},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	reporter := &TestProgressReporter{}

	linter, err := NewConcurrentLinter(cfg, logger, fs,
		WithWorkerCount(2),
		WithProgressReporter(reporter))
	require.NoError(t, err)

	violations, err := linter.LintWithContext(context.Background(), ".")
	require.NoError(t, err)
	assert.NotNil(t, violations)

	// Check progress was reported
	assert.Greater(t, len(reporter.filesStarted), 0)
	assert.Greater(t, len(reporter.filesCompleted), 0)
	assert.Greater(t, len(reporter.progressUpdates), 0)
	assert.NotNil(t, reporter.stats)

	// All started files should be completed
	assert.Equal(t, len(reporter.filesStarted), len(reporter.filesCompleted))
}

func TestConcurrentVsSequential(t *testing.T) {
	fs := afero.NewMemMapFs()
	setupLargeProject(t, fs, 50)

	cfg := Config{
		Modfile: "go.mod",
		Rules: []Rule{
			{
				Path: "pkg",
				Allowed: []string{"fmt", "context", "errors"},
				Prohibited: []ProhibitedPkg{
					{Name: "internal/database", Cause: "No database in pkg"},
				},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Run sequential
	sequential, err := NewLinter(cfg, logger, fs)
	require.NoError(t, err)

	seqViolations, err := sequential.Lint(".")
	require.NoError(t, err)

	// Run concurrent
	concurrent, err := NewConcurrentLinter(cfg, logger, fs, WithWorkerCount(4))
	require.NoError(t, err)

	concViolations, err := concurrent.LintWithContext(context.Background(), ".")
	require.NoError(t, err)

	// Results should be the same
	assert.Equal(t, len(seqViolations.Violations), len(concViolations.Violations))
}

func TestConcurrentLinterRaceCondition(t *testing.T) {
	// This test runs with -race flag to detect race conditions
	fs := afero.NewMemMapFs()
	setupLargeProject(t, fs, 100)

	cfg := Config{
		Modfile: "go.mod",
		Rules: []Rule{
			{
				Path: "pkg",
				Allowed: []string{"fmt"},
				Prohibited: []ProhibitedPkg{
					{Name: "unsafe", Cause: "No unsafe"},
					{Name: "reflect", Cause: "No reflect"},
				},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Run with maximum concurrency to trigger potential races
	linter, err := NewConcurrentLinter(cfg, logger, fs, WithWorkerCount(16))
	require.NoError(t, err)

	// Run multiple times to increase chance of detecting races
	for i := 0; i < 5; i++ {
		violations, err := linter.LintWithContext(context.Background(), ".")
		require.NoError(t, err)
		assert.NotNil(t, violations)
	}
}

func TestLintStats(t *testing.T) {
	stats := &LintStats{
		startTime: time.Now(),
	}

	stats.filesProcessed.Store(100)
	stats.totalFiles.Store(150)

	// Test duration calculation
	time.Sleep(100 * time.Millisecond)
	stats.endTime = time.Now()

	duration := stats.Duration()
	assert.Greater(t, duration.Milliseconds(), int64(90))
	assert.Less(t, duration.Milliseconds(), int64(200))

	// Test files per second
	fps := stats.FilesPerSecond()
	assert.Greater(t, fps, float64(0))
}

func TestConcurrentLinterErrorHandling(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create invalid Go file
	_ = afero.WriteFile(fs, "invalid.go", []byte("not valid go code {{{"), 0644)
	_ = afero.WriteFile(fs, "go.mod", []byte("module test\n\ngo 1.21\n"), 0644)

	cfg := Config{
		Modfile: "go.mod",
		Rules: []Rule{
			{
				Path: ".",
				Allowed: []string{"fmt"},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	linter, err := NewConcurrentLinter(cfg, logger, fs)
	require.NoError(t, err)

	// Should handle parse errors gracefully
	violations, err := linter.LintWithContext(context.Background(), ".")
	assert.NoError(t, err) // Should not return error, just skip invalid files
	assert.NotNil(t, violations)
}

// Benchmark tests

func BenchmarkConcurrentLinter(b *testing.B) {
	testCases := []struct {
		name        string
		numFiles    int
		workerCount int
	}{
		{"10_files_1_worker", 10, 1},
		{"10_files_4_workers", 10, 4},
		{"100_files_1_worker", 100, 1},
		{"100_files_4_workers", 100, 4},
		{"100_files_8_workers", 100, 8},
		{"500_files_1_worker", 500, 1},
		{"500_files_8_workers", 500, 8},
		{"500_files_16_workers", 500, 16},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			fs := setupBenchmarkFs(tc.numFiles)
			cfg := setupBenchmarkConfig()
			logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

			linter, err := NewConcurrentLinter(cfg, logger, fs,
				WithWorkerCount(tc.workerCount))
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := linter.LintWithContext(context.Background(), ".")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Helper functions

func setupTestProject(t *testing.T, fs afero.Fs) {
	// Create go.mod
	modContent := `module testproject

go 1.21
`
	err := afero.WriteFile(fs, "go.mod", []byte(modContent), 0644)
	require.NoError(t, err)

	// Create API file with violation
	apiContent := `package api

import (
	"fmt"
	"internal/database"
)

func Handler() {
	fmt.Println("API Handler")
	database.Query()
}
`
	err = fs.MkdirAll("internal/api", 0755)
	require.NoError(t, err)
	err = afero.WriteFile(fs, "internal/api/handler.go", []byte(apiContent), 0644)
	require.NoError(t, err)

	// Create database file
	dbContent := `package database

func Query() {
	// Database query
}
`
	err = fs.MkdirAll("internal/database", 0755)
	require.NoError(t, err)
	err = afero.WriteFile(fs, "internal/database/db.go", []byte(dbContent), 0644)
	require.NoError(t, err)
}

func setupLargeProject(t *testing.T, fs afero.Fs, numFiles int) {
	// Create go.mod
	modContent := `module largeproject

go 1.21
`
	err := afero.WriteFile(fs, "go.mod", []byte(modContent), 0644)
	require.NoError(t, err)

	// Create many files
	for i := 0; i < numFiles; i++ {
		dir := fmt.Sprintf("pkg/module%d", i/10)
		filename := fmt.Sprintf("%s/file%d.go", dir, i)

		content := fmt.Sprintf(`package module%d

import (
	"fmt"
	"context"
	"errors"
	"internal/database"
)

func Function%d() {
	fmt.Println("Function %d")
}
`, i/10, i, i)

		err = fs.MkdirAll(dir, 0755)
		require.NoError(t, err)
		err = afero.WriteFile(fs, filename, []byte(content), 0644)
		require.NoError(t, err)
	}
}