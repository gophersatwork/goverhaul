package goverhaul

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gophersatwork/granular"
	"github.com/spf13/afero"
)

func TestNewCache(t *testing.T) {
	cacheDir := "/tmp/cache"
	memFs := NewCacheFs(t, cacheDir)

	cachePath := filepath.Join(cacheDir, "cache.json")

	// Test creating a new cache
	cache, err := granular.New(cacheDir, granular.WithFs(memFs))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	if cache == nil {
		t.Fatal("Expected non-nil cache, got nil")
	}

	// Verify the cache file was created
	exists, err := afero.Exists(memFs, cachePath)
	if err != nil && !exists {
		t.Fatal("Expected non-nil cache, got nil")
	}
}

func TestLintCache_AddFile(t *testing.T) {
	cacheDir := "/tmp/cache"
	memFs := NewCacheFs(t, cacheDir)

	cachePath := filepath.Join(cacheDir, "cache.json")

	// Create a test file
	testPath := filepath.Join(cacheDir, "main.go")
	err := afero.WriteFile(memFs, testPath, []byte("package main\n\nfunc main() {}\n"), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a new cache
	lintCache, err := NewCacheWithFs(cachePath, memFs)
	if err != nil {
		t.Fatalf("Failed to create granular cache: %v", err)
	}

	// printDirTree(memFs, tempDir)

	// Test adding a file to the cache
	err = lintCache.AddFile(testPath)
	if err != nil {
		t.Fatalf("Failed to add file to cache: %v", err)
	}

	// Verify the file was added to the cache

	key := granular.Key{
		Inputs: []granular.Input{granular.FileInput{
			Path: testPath,
			Fs:   memFs,
		}},
	}
	result, found, err := lintCache.gCache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}
	if !found {
		t.Errorf("File %s was not found in cache", testPath)
	}
	fmt.Printf("%+v\n", result)
}

func TestLintCache_AddFileWithViolations(t *testing.T) {
	cacheDir := "/tmp/cache"
	memFs := NewCacheFs(t, cacheDir)
	cachePath := filepath.Join(cacheDir, "cache.json")

	// Create a test file
	testPath := filepath.Join(cacheDir, "test.go")
	err := afero.WriteFile(memFs, testPath, []byte("package main\n\nimport \"prohibited/package\"\n\nfunc main() {}\n"), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a new cache
	lintCache, err := NewCacheWithFs(cachePath, memFs)
	if err != nil {
		t.Fatalf("Failed to create granular cache: %v", err)
	}

	// printDirTree(memFs, tempDir)

	// Create test violations
	violations := []LintViolation{
		{
			File:   testPath,
			Import: "prohibited/package",
			Cause:  "This import is prohibited",
			Rule:   "test-rule",
		},
	}

	// Test adding a file with violations to the cache
	err = lintCache.AddFileWithViolations(testPath, violations)
	if err != nil {
		t.Fatalf("Failed to add file with violations to cache: %v", err)
	}

	// Verify the file was added to the cache with violations
	normalizedPath := NormalizePath(testPath)
	key := granular.Key{
		Inputs: []granular.Input{granular.FileInput{
			Path: normalizedPath,
			Fs:   memFs,
		}},
	}
	result, found, err := lintCache.gCache.Get(key)
	if err != nil {
		t.Fatalf("Failed to get from cache: %v", err)
	}
	if !found {
		t.Errorf("File %s was not found in cache", testPath)
	}

	// Verify the violations were stored correctly
	violationsStr, ok := result.Metadata["violations"]
	if !ok {
		t.Error("Violations metadata not found in cache entry")
	}

	var cachedViolations LintViolations
	err = json.Unmarshal([]byte(violationsStr), &cachedViolations)
	if err != nil {
		t.Fatalf("Failed to unmarshal cached violations: %v", err)
	}

	if len(cachedViolations.Violations) != len(violations) {
		t.Errorf("Expected %d violations, got %d", len(violations), len(cachedViolations.Violations))
	}

	if cachedViolations.Violations[0].Import != violations[0].Import {
		t.Errorf("Expected import %s, got %s", violations[0].Import, cachedViolations.Violations[0].Import)
	}
}

func TestLintCache_HasEntry(t *testing.T) {
	cacheDir := "/tmp/cache"
	memFs := NewCacheFs(t, cacheDir)

	cachePath := filepath.Join(cacheDir, "cache.json")

	// Create a test file
	testPath := filepath.Join(cacheDir, "test.go")
	err := afero.WriteFile(memFs, testPath, []byte("package main\n\nfunc main() {}\n"), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a new cache
	lintCache, err := NewCacheWithFs(cachePath, memFs)
	if err != nil {
		t.Fatalf("Failed to create granular cache: %v", err)
	}

	t.Run("file not in cache", func(t *testing.T) {
		_, err = lintCache.HasEntry(testPath)
		if !errors.Is(err, ErrEntryNotFound) {
			t.Errorf("Expected ErrEntryNotFound for non-existent file, got %v", err)
		}
	})

	t.Run("Add file to cache without violations", func(t *testing.T) {
		err = lintCache.AddFile(testPath)
		if err != nil {
			t.Fatalf("Failed to add file to cache: %v", err)
		}

		// Verify file is in cache without violations
		violations, err := lintCache.HasEntry(testPath)
		if err != nil {
			t.Errorf("Unexpected error checking cache entry: %v", err)
		}
		if !violations.IsEmpty() {
			t.Errorf("Expected empty violations, got %d violations", len(violations.Violations))
		}
	})

	t.Run("Add file to cache with violations", func(t *testing.T) {
		testViolations := []LintViolation{
			{
				File:   testPath,
				Import: "prohibited/package",
				Cause:  "This import is prohibited",
				Rule:   "test-rule",
			},
		}

		err = lintCache.AddFileWithViolations(testPath, testViolations)
		if err != nil {
			t.Fatalf("Failed to add file with violations to cache: %v", err)
		}

		// Verify file is in cache with violations
		violations, err := lintCache.HasEntry(testPath)
		if err != nil {
			t.Errorf("Unexpected error checking cache entry: %v", err)
		}
		if violations.IsEmpty() {
			t.Errorf("Expected violations, got empty violations")
		}
		if len(violations.Violations) != len(testViolations) {
			t.Errorf("Expected %d violations, got %d", len(testViolations), len(violations.Violations))
		}
		if violations.Violations[0].Import != testViolations[0].Import {
			t.Errorf("Expected import %s, got %s", testViolations[0].Import, violations.Violations[0].Import)
		}
		if violations.Violations[0].Cached {
			t.Errorf("Expected cached %v, got %s", true, violations.Violations[0].Import)
		}
	})

	t.Run("invalid violations data", func(t *testing.T) {
		// Create a key directly with invalid metadata
		normalizedPath := NormalizePath(testPath)
		key := granular.Key{
			Inputs: []granular.Input{granular.FileInput{
				Path: normalizedPath,
				Fs:   memFs,
			}},
		}

		// Store invalid JSON in the violations metadata
		metadata := make(map[string]string)
		metadata["violations"] = "invalid json"
		result := granular.Result{
			Metadata: metadata,
		}

		err = lintCache.gCache.Store(key, result)
		if err != nil {
			t.Fatalf("Failed to store invalid violations: %v", err)
		}

		// Verify error when reading invalid violations
		_, err = lintCache.HasEntry(testPath)
		if !errors.Is(err, ErrReadingCachedViolations) {
			t.Errorf("Expected ErrReadingCachedViolations for invalid violations, got %v", err)
		}
	})
}

func NewCacheFs(t *testing.T, cacheDir string) afero.Fs {
	t.Helper()

	memFs := afero.NewMemMapFs()
	err := memFs.Mkdir(cacheDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	return memFs
}

func printDirTree(fs afero.Fs, path string) error {
	err := afero.Walk(fs, path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if p == path {
			return nil
		}

		depth := strings.Count(p, string(os.PathSeparator))
		indent := strings.Repeat("│   ", depth-1)

		name := info.Name()
		if info.IsDir() {
			fmt.Printf("%s├── 📁 %s\n", indent, name)
		} else {
			fmt.Printf("%s├── 📄 %s\n", indent, name)
		}

		return nil
	})
	if err != nil {
		log.Fatalf("Failed to inspect the folder: %v", err)
	}

	return nil
}
