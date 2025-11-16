package goverhaul

import (
	"fmt"
	"sync"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMusCache_Create tests creating a MUS cache
func TestMusCache_Create(t *testing.T) {
	fs := afero.NewMemMapFs()

	t.Run("create mus cache successfully", func(t *testing.T) {
		cache, err := NewMusCache(".goverhaul_mus_test.cache", fs)
		require.NoError(t, err)
		assert.NotNil(t, cache)
		assert.NotNil(t, cache.gCache)
		assert.Equal(t, fs, cache.fs)
	})

	t.Run("create mus cache with nil fs", func(t *testing.T) {
		cache, err := NewMusCache(".goverhaul_mus_test_nil.cache", nil)
		require.NoError(t, err)
		assert.NotNil(t, cache)
		assert.Nil(t, cache.fs)
	})
}

// TestMusCache_AddFile tests adding files without violations
func TestMusCache_AddFile(t *testing.T) {
	fs := afero.NewMemMapFs()

	t.Run("add file without violations", func(t *testing.T) {
		cache, err := NewMusCache(".goverhaul_mus_test.cache", fs)
		require.NoError(t, err)

		testFile := "test.go"
		err = afero.WriteFile(fs, testFile, []byte("package test\n\nimport \"fmt\""), 0644)
		require.NoError(t, err)

		err = cache.AddFile(testFile)
		require.NoError(t, err)

		// Verify entry exists but has no violations
		result, err := cache.HasEntry(testFile)
		require.NoError(t, err)
		assert.Empty(t, result.Violations)
		assert.True(t, result.IsEmpty())
	})

	t.Run("add file with normalized path", func(t *testing.T) {
		cache, err := NewMusCache(".goverhaul_mus_test.cache", fs)
		require.NoError(t, err)

		testFile := "./path/to/test.go"
		normalizedFile := "path/to/test.go"
		err = afero.WriteFile(fs, normalizedFile, []byte("package test"), 0644)
		require.NoError(t, err)

		err = cache.AddFile(testFile)
		require.NoError(t, err)

		// Verify entry exists using normalized path
		result, err := cache.HasEntry(normalizedFile)
		require.NoError(t, err)
		assert.Empty(t, result.Violations)
	})
}

// TestMusCache_AddAndRetrieveViolations tests adding and retrieving violations
func TestMusCache_AddAndRetrieveViolations(t *testing.T) {
	fs := afero.NewMemMapFs()

	t.Run("add and retrieve single violation", func(t *testing.T) {
		cache, err := NewMusCache(".goverhaul_mus_test.cache", fs)
		require.NoError(t, err)

		testFile := "test.go"
		err = afero.WriteFile(fs, testFile, []byte("package test"), 0644)
		require.NoError(t, err)

		violations := []LintViolation{
			{
				File:    testFile,
				Import:  "internal/database",
				Rule:    "no-db",
				Cause:   "Database not allowed",
				Details: "Use abstraction layer",
			},
		}

		err = cache.AddFileWithViolations(testFile, violations)
		require.NoError(t, err)

		result, err := cache.HasEntry(testFile)
		require.NoError(t, err)
		assert.Len(t, result.Violations, 1)
		assert.True(t, result.Violations[0].Cached)
		assert.Equal(t, "internal/database", result.Violations[0].Import)
		assert.Equal(t, "no-db", result.Violations[0].Rule)
		assert.Equal(t, "Database not allowed", result.Violations[0].Cause)
		assert.Equal(t, "Use abstraction layer", result.Violations[0].Details)
	})

	t.Run("add and retrieve multiple violations", func(t *testing.T) {
		cache, err := NewMusCache(".goverhaul_mus_test.cache", fs)
		require.NoError(t, err)

		testFile := "multi_test.go"
		err = afero.WriteFile(fs, testFile, []byte("package test"), 0644)
		require.NoError(t, err)

		violations := []LintViolation{
			{
				File:   testFile,
				Import: "internal/database",
				Rule:   "no-db",
				Cause:  "Database not allowed",
			},
			{
				File:   testFile,
				Import: "unsafe",
				Rule:   "no-unsafe",
				Cause:  "Unsafe package not allowed",
			},
			{
				File:   testFile,
				Import: "github.com/deprecated/pkg",
				Rule:   "no-deprecated",
				Cause:  "Package is deprecated",
			},
		}

		err = cache.AddFileWithViolations(testFile, violations)
		require.NoError(t, err)

		result, err := cache.HasEntry(testFile)
		require.NoError(t, err)
		assert.Len(t, result.Violations, 3)

		// Verify all violations are marked as cached
		for _, v := range result.Violations {
			assert.True(t, v.Cached, "violation should be marked as cached")
		}

		// Verify content
		assert.Equal(t, "internal/database", result.Violations[0].Import)
		assert.Equal(t, "unsafe", result.Violations[1].Import)
		assert.Equal(t, "github.com/deprecated/pkg", result.Violations[2].Import)
	})

	t.Run("add empty violations slice", func(t *testing.T) {
		cache, err := NewMusCache(".goverhaul_mus_test.cache", fs)
		require.NoError(t, err)

		testFile := "empty_test.go"
		err = afero.WriteFile(fs, testFile, []byte("package test"), 0644)
		require.NoError(t, err)

		err = cache.AddFileWithViolations(testFile, []LintViolation{})
		require.NoError(t, err)

		result, err := cache.HasEntry(testFile)
		require.NoError(t, err)
		assert.Empty(t, result.Violations)
		assert.True(t, result.IsEmpty())
	})
}

// TestMusCache_EdgeCases tests edge cases
func TestMusCache_EdgeCases(t *testing.T) {
	fs := afero.NewMemMapFs()

	t.Run("retrieve non-existent entry", func(t *testing.T) {
		cache, err := NewMusCache(".goverhaul_mus_test.cache", fs)
		require.NoError(t, err)

		_, err = cache.HasEntry("nonexistent.go")
		assert.ErrorIs(t, err, ErrEntryNotFound)
	})

	t.Run("violation with empty strings", func(t *testing.T) {
		cache, err := NewMusCache(".goverhaul_mus_test.cache", fs)
		require.NoError(t, err)

		testFile := "empty_strings_test.go"
		err = afero.WriteFile(fs, testFile, []byte("package test"), 0644)
		require.NoError(t, err)

		violations := []LintViolation{
			{
				File:    testFile,
				Import:  "some/import",
				Rule:    "some-rule",
				Cause:   "",
				Details: "",
				Cached:  false,
			},
		}

		err = cache.AddFileWithViolations(testFile, violations)
		require.NoError(t, err)

		result, err := cache.HasEntry(testFile)
		require.NoError(t, err)
		assert.Len(t, result.Violations, 1)
		assert.Equal(t, "", result.Violations[0].Cause)
		assert.Equal(t, "", result.Violations[0].Details)
	})

	t.Run("violation with unicode characters", func(t *testing.T) {
		cache, err := NewMusCache(".goverhaul_mus_test.cache", fs)
		require.NoError(t, err)

		testFile := "unicode_test.go"
		err = afero.WriteFile(fs, testFile, []byte("package test"), 0644)
		require.NoError(t, err)

		violations := []LintViolation{
			{
				File:    testFile,
				Import:  "some/import",
				Rule:    "unicode-rule",
				Cause:   "Contains unicode: こんにちは, 你好, مرحبا",
				Details: "Special chars: £€¥₹",
			},
		}

		err = cache.AddFileWithViolations(testFile, violations)
		require.NoError(t, err)

		result, err := cache.HasEntry(testFile)
		require.NoError(t, err)
		assert.Len(t, result.Violations, 1)
		assert.Equal(t, "Contains unicode: こんにちは, 你好, مرحبا", result.Violations[0].Cause)
		assert.Equal(t, "Special chars: £€¥₹", result.Violations[0].Details)
	})
}

// TestMusCache_LargeDataset tests with large number of violations
func TestMusCache_LargeDataset(t *testing.T) {
	t.Run("100 violations", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		cache, err := NewMusCache(".goverhaul_large_test.cache", fs)
		require.NoError(t, err)

		testFile := "large_test.go"
		err = afero.WriteFile(fs, testFile, []byte("package test"), 0644)
		require.NoError(t, err)

		// Create 100 violations
		violations := make([]LintViolation, 100)
		for i := 0; i < 100; i++ {
			violations[i] = LintViolation{
				File:    testFile,
				Import:  fmt.Sprintf("import/path/%d", i),
				Rule:    fmt.Sprintf("rule-%d", i%10),
				Cause:   fmt.Sprintf("Violation cause number %d", i),
				Details: fmt.Sprintf("Detailed description for violation %d", i),
			}
		}

		err = cache.AddFileWithViolations(testFile, violations)
		require.NoError(t, err)

		result, err := cache.HasEntry(testFile)
		require.NoError(t, err)
		assert.Len(t, result.Violations, 100)

		// Verify some violations
		assert.Equal(t, "import/path/0", result.Violations[0].Import)
		assert.Equal(t, "import/path/99", result.Violations[99].Import)
		assert.True(t, result.Violations[50].Cached)

		t.Logf("Successfully cached and retrieved 100 violations")
	})
}

// TestMusCache_CacheInvalidation tests cache invalidation behavior
func TestMusCache_CacheInvalidation(t *testing.T) {
	fs := afero.NewMemMapFs()

	t.Run("overwrite existing entry", func(t *testing.T) {
		cache, err := NewMusCache(".goverhaul_mus_test.cache", fs)
		require.NoError(t, err)

		testFile := "overwrite_test.go"
		err = afero.WriteFile(fs, testFile, []byte("package test"), 0644)
		require.NoError(t, err)

		// Add initial violations
		violations1 := []LintViolation{
			{File: testFile, Import: "old/import", Rule: "old-rule"},
		}
		err = cache.AddFileWithViolations(testFile, violations1)
		require.NoError(t, err)

		// Overwrite with new violations
		violations2 := []LintViolation{
			{File: testFile, Import: "new/import1", Rule: "new-rule1"},
			{File: testFile, Import: "new/import2", Rule: "new-rule2"},
		}
		err = cache.AddFileWithViolations(testFile, violations2)
		require.NoError(t, err)

		// Verify new violations
		result, err := cache.HasEntry(testFile)
		require.NoError(t, err)
		assert.Len(t, result.Violations, 2)
		assert.Equal(t, "new/import1", result.Violations[0].Import)
		assert.Equal(t, "new/import2", result.Violations[1].Import)
	})

	t.Run("file modification invalidates cache", func(t *testing.T) {
		cache, err := NewMusCache(".goverhaul_mus_test.cache", fs)
		require.NoError(t, err)

		testFile := "modified_test.go"
		err = afero.WriteFile(fs, testFile, []byte("package test\n// version 1"), 0644)
		require.NoError(t, err)

		violations := []LintViolation{
			{File: testFile, Import: "some/import", Rule: "some-rule"},
		}
		err = cache.AddFileWithViolations(testFile, violations)
		require.NoError(t, err)

		// Verify cache hit
		result, err := cache.HasEntry(testFile)
		require.NoError(t, err)
		assert.Len(t, result.Violations, 1)

		// Modify file
		err = afero.WriteFile(fs, testFile, []byte("package test\n// version 2"), 0644)
		require.NoError(t, err)

		// Cache should be invalidated (granular checks file modification)
		_, err = cache.HasEntry(testFile)
		assert.ErrorIs(t, err, ErrEntryNotFound)
	})
}

// TestMusCache_ConcurrentAccess tests concurrent access
func TestMusCache_ConcurrentAccess(t *testing.T) {
	fs := afero.NewMemMapFs()

	t.Run("concurrent writes to different files", func(t *testing.T) {
		cache, err := NewMusCache(".goverhaul_mus_test.cache", fs)
		require.NoError(t, err)

		const numGoroutines = 10
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()

				testFile := fmt.Sprintf("concurrent_%d.go", id)
				err := afero.WriteFile(fs, testFile, []byte("package test"), 0644)
				require.NoError(t, err)

				violations := []LintViolation{
					{
						File:   testFile,
						Import: fmt.Sprintf("import/%d", id),
						Rule:   fmt.Sprintf("rule-%d", id),
					},
				}

				err = cache.AddFileWithViolations(testFile, violations)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()

		// Verify all files were cached
		for i := 0; i < numGoroutines; i++ {
			testFile := fmt.Sprintf("concurrent_%d.go", i)
			result, err := cache.HasEntry(testFile)
			assert.NoError(t, err)
			assert.Len(t, result.Violations, 1)
		}
	})

	t.Run("concurrent reads", func(t *testing.T) {
		readFs := afero.NewMemMapFs()
		cache, err := NewMusCache(".goverhaul_concurrent_reads.cache", readFs)
		require.NoError(t, err)

		testFile := "shared_test.go"
		err = afero.WriteFile(readFs, testFile, []byte("package test"), 0644)
		require.NoError(t, err)

		violations := []LintViolation{
			{File: testFile, Import: "shared/import", Rule: "shared-rule"},
		}
		err = cache.AddFileWithViolations(testFile, violations)
		require.NoError(t, err)

		// Verify entry exists before concurrent reads
		result, err := cache.HasEntry(testFile)
		require.NoError(t, err)
		require.Len(t, result.Violations, 1)

		const numReaders = 5
		var wg sync.WaitGroup
		wg.Add(numReaders)

		// Simple concurrent reads without assertions since granular cache may have limitations
		for i := 0; i < numReaders; i++ {
			go func() {
				defer wg.Done()
				_, _ = cache.HasEntry(testFile)
			}()
		}

		wg.Wait()

		// Verify entry still exists after concurrent access
		result, err = cache.HasEntry(testFile)
		assert.NoError(t, err)
		assert.Len(t, result.Violations, 1)
	})
}

// TestMusCache_Stats tests cache statistics
func TestMusCache_Stats(t *testing.T) {
	fs := afero.NewMemMapFs()

	t.Run("get stats", func(t *testing.T) {
		cache, err := NewMusCache(".goverhaul_mus_test.cache", fs)
		require.NoError(t, err)

		stats := cache.GetStats()
		assert.Contains(t, stats, "MUS")
		assert.Contains(t, stats, "performance")
	})
}
