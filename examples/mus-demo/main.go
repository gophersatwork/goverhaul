package main

import (
	"fmt"
	"log"
	"path/filepath"
	
	"github.com/gophersatwork/goverhaul"
	"github.com/spf13/afero"
)

func main() {
	// Create an in-memory filesystem
	fs := afero.NewMemMapFs()
	
	// Create cache directory
	cacheDir := "/tmp/test-cache"
	if err := fs.MkdirAll(cacheDir, 0755); err != nil {
		log.Fatalf("Failed to create cache dir: %v", err)
	}
	
	// Create a test Go file
	testFile := filepath.Join(cacheDir, "test.go")
	content := []byte(`package main

import "fmt"
import "unsafe"

func main() {
	fmt.Println("Hello")
}
`)
	if err := afero.WriteFile(fs, testFile, content, 0644); err != nil {
		log.Fatalf("Failed to write test file: %v", err)
	}
	
	// Create MUS cache
	cache, err := goverhaul.NewMusCache(filepath.Join(cacheDir, "cache.db"), fs)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	
	// Create some test violations
	violations := []goverhaul.LintViolation{
		{
			File:    testFile,
			Import:  "unsafe",
			Rule:    "no-unsafe",
			Cause:   "unsafe package is prohibited",
			Details: "Using unsafe can lead to undefined behavior",
		},
		{
			File:    testFile,
			Import:  "fmt",
			Rule:    "fmt-check",
			Cause:   "direct fmt usage not recommended",
			Details: "Use logging library instead",
		},
	}
	
	// Add violations to cache
	if err := cache.AddFileWithViolations(testFile, violations); err != nil {
		log.Fatalf("Failed to add violations: %v", err)
	}
	
	fmt.Println("✓ Added violations to cache")
	
	// Retrieve violations from cache
	cached, err := cache.HasEntry(testFile)
	if err != nil {
		log.Fatalf("Failed to retrieve violations: %v", err)
	}
	
	fmt.Printf("✓ Retrieved %d violations from cache\n", len(cached.Violations))
	
	// Verify violations
	if len(cached.Violations) != 2 {
		log.Fatalf("Expected 2 violations, got %d", len(cached.Violations))
	}
	
	for i, v := range cached.Violations {
		if !v.Cached {
			log.Fatalf("Violation %d not marked as cached", i)
		}
		fmt.Printf("  - Violation %d: %s in %s\n", i+1, v.Import, v.Rule)
	}
	
	// Check stats
	stats := cache.GetStats()
	fmt.Printf("✓ Cache stats: %s\n", stats)
	
	fmt.Println("\n✅ All MUS cache tests passed!")
}
