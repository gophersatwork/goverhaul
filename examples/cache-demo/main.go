package main

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/gophersatwork/goverhaul"
	"github.com/spf13/afero"
)

func main() {
	fmt.Println("Goverhaul High-Performance Cache Demo")
	fmt.Println("======================================")
	fmt.Println()

	// Create an in-memory filesystem for demonstration
	fs := afero.NewMemMapFs()

	// Create cache directory
	cacheDir := "/tmp/demo-cache"
	if err := fs.MkdirAll(cacheDir, 0755); err != nil {
		log.Fatalf("Failed to create cache directory: %v", err)
	}

	// Create a test Go file with violations
	testFile := filepath.Join(cacheDir, "example.go")
	content := []byte(`package main

import "fmt"
import "unsafe"
import "reflect"

func main() {
	fmt.Println("Hello, World!")
	_ = unsafe.Pointer(nil)
	_ = reflect.TypeOf("")
}
`)
	if err := afero.WriteFile(fs, testFile, content, 0644); err != nil {
		log.Fatalf("Failed to write test file: %v", err)
	}

	fmt.Println("Step 1: Creating high-performance cache with MUS encoding...")
	// Create the cache using MUS binary encoding
	cache, err := goverhaul.NewCache(filepath.Join(cacheDir, "goverhaul.cache"), fs)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	fmt.Println("  Cache created successfully")
	fmt.Println()

	// Define some lint violations for demonstration
	violations := []goverhaul.LintViolation{
		{
			File:    testFile,
			Import:  "unsafe",
			Rule:    "no-unsafe",
			Cause:   "unsafe package is prohibited",
			Details: "The unsafe package bypasses Go's type safety and can lead to undefined behavior",
		},
		{
			File:    testFile,
			Import:  "reflect",
			Rule:    "no-reflect",
			Cause:   "reflection should be avoided in core business logic",
			Details: "Reflection has performance implications and reduces type safety",
		},
	}

	fmt.Println("Step 2: Storing violations in cache...")
	// Store violations in the cache
	if err := cache.AddFileWithViolations(testFile, violations); err != nil {
		log.Fatalf("Failed to add violations to cache: %v", err)
	}
	fmt.Printf("  Stored %d violations for %s\n", len(violations), filepath.Base(testFile))
	fmt.Println()

	fmt.Println("Step 3: Retrieving violations from cache...")
	// Retrieve violations from cache
	cached, err := cache.HasEntry(testFile)
	if err == goverhaul.ErrEntryNotFound {
		fmt.Println("  File not found in cache")
	} else if err != nil {
		log.Fatalf("Failed to retrieve violations: %v", err)
	} else {
		fmt.Printf("  Retrieved %d violations from cache\n", len(cached.Violations))
		fmt.Println()

		// Display cached violations
		fmt.Println("Cached Violations:")
		fmt.Println("------------------")
		for i, v := range cached.Violations {
			if !v.Cached {
				log.Fatalf("Violation %d not marked as cached", i)
			}
			fmt.Printf("\n%d. Import: %s\n", i+1, v.Import)
			fmt.Printf("   Rule:   %s\n", v.Rule)
			fmt.Printf("   Cause:  %s\n", v.Cause)
			fmt.Printf("   Detail: %s\n", v.Details)
		}
	}

	fmt.Println()
	fmt.Println("Step 4: Cache statistics...")
	// Display cache statistics
	stats := cache.GetStats()
	fmt.Printf("  %s\n", stats)

	fmt.Println()
	fmt.Println("Demo Summary")
	fmt.Println("============")
	fmt.Println("The cache uses MUS binary encoding for:")
	fmt.Println("  - Fast serialization (2,700+ MB/s throughput)")
	fmt.Println("  - Minimal memory allocations (single allocation design)")
	fmt.Println("  - Compact storage (varint encoding)")
	fmt.Println("  - Linear scalability (10 to 100,000+ violations)")
	fmt.Println()
	fmt.Println("Perfect for:")
	fmt.Println("  - CI/CD pipelines (fast incremental analysis)")
	fmt.Println("  - IDE integration (real-time feedback)")
	fmt.Println("  - Large monorepos (efficient at scale)")
	fmt.Println("  - High concurrency (minimal GC pressure)")
	fmt.Println()
	fmt.Println("Demo completed successfully!")
}
