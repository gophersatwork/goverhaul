package goverhaul

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
)

// generateViolations creates test violations for benchmarking
func generateViolations(count int) []LintViolation {
	violations := make([]LintViolation, count)
	for i := range violations {
		violations[i] = LintViolation{
			File:    fmt.Sprintf("internal/pkg/module%d/file%d.go", i/100, i),
			Import:  fmt.Sprintf("internal/prohibited/package%d", i),
			Rule:    fmt.Sprintf("layer-rule-%d", i%10),
			Cause:   fmt.Sprintf("Package violates architectural boundary rule %d", i%10),
			Details: fmt.Sprintf("Additional context: this import creates a dependency cycle in module %d", i/100),
			Cached:  false,
		}
	}
	return violations
}

// wrapViolations wraps violations in LintViolations struct
func wrapViolations(violations []LintViolation) LintViolations {
	return LintViolations{
		Violations: violations,
	}
}

// BenchmarkEncoding_Encode benchmarks encoding performance
func BenchmarkEncoding_Encode(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000, 100000}

	for _, size := range sizes {
		violations := wrapViolations(generateViolations(size))

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			var result []byte
			var err error
			for i := 0; i < b.N; i++ {
				result, err = marshalLintViolations(violations)
				if err != nil {
					b.Fatalf("Failed to encode: %v", err)
				}
			}

			// Report metrics
			b.SetBytes(int64(len(result)))
			b.ReportMetric(float64(len(result)), "bytes/op")
		})
	}
}

// BenchmarkEncoding_Decode benchmarks decoding performance
func BenchmarkEncoding_Decode(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000, 100000}

	for _, size := range sizes {
		violations := wrapViolations(generateViolations(size))
		encoded, err := marshalLintViolations(violations)
		if err != nil {
			b.Fatalf("Failed to encode: %v", err)
		}

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := unmarshalLintViolations(encoded)
				if err != nil {
					b.Fatalf("Failed to decode: %v", err)
				}
			}

			b.SetBytes(int64(len(encoded)))
		})
	}
}

// BenchmarkEncoding_RoundTrip benchmarks complete encode/decode cycle
func BenchmarkEncoding_RoundTrip(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000, 100000}

	for _, size := range sizes {
		violations := wrapViolations(generateViolations(size))

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				encoded, err := marshalLintViolations(violations)
				if err != nil {
					b.Fatalf("Failed to encode: %v", err)
				}

				_, err = unmarshalLintViolations(encoded)
				if err != nil {
					b.Fatalf("Failed to decode: %v", err)
				}
			}

			b.SetBytes(int64(size))
		})
	}
}

// BenchmarkSerializedSize reports serialized sizes at different scales
func BenchmarkSerializedSize(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000, 100000}

	for _, size := range sizes {
		violations := wrapViolations(generateViolations(size))

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			encoded, err := marshalLintViolations(violations)
			if err != nil {
				b.Fatalf("Failed to encode: %v", err)
			}
			b.ReportMetric(float64(len(encoded)), "bytes")
			b.ReportMetric(float64(len(encoded))/float64(size), "bytes/violation")
		})
	}
}

// BenchmarkCacheWrite benchmarks cache write operations
func BenchmarkCacheWrite(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		violations := generateViolations(size)

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				fs := afero.NewMemMapFs()
				cache, err := NewCache(b.TempDir(), fs)
				if err != nil {
					b.Fatalf("Failed to create cache: %v", err)
				}
				testPath := fmt.Sprintf("test_file_%d.go", i)
				// Create the test file in the filesystem
				_ = afero.WriteFile(fs, testPath, []byte("package test"), 0644)
				b.StartTimer()

				err = cache.AddFileWithViolations(testPath, violations)
				if err != nil {
					b.Fatalf("Failed to add violations: %v", err)
				}
			}

			b.SetBytes(int64(size))
		})
	}
}

// BenchmarkCacheRead benchmarks cache read operations
func BenchmarkCacheRead(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		violations := generateViolations(size)

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			// Setup: create cache and populate it
			fs := afero.NewMemMapFs()
			cache, err := NewCache(b.TempDir(), fs)
			if err != nil {
				b.Fatalf("Failed to create cache: %v", err)
			}

			testPath := "test_file.go"
			// Create the test file in the filesystem
			_ = afero.WriteFile(fs, testPath, []byte("package test"), 0644)
			err = cache.AddFileWithViolations(testPath, violations)
			if err != nil {
				b.Fatalf("Failed to add violations: %v", err)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := cache.HasEntry(testPath)
				if err != nil {
					b.Fatalf("Failed to read violations: %v", err)
				}
			}

			b.SetBytes(int64(size))
		})
	}
}

// BenchmarkCacheRoundTrip benchmarks complete cache write+read cycle
func BenchmarkCacheRoundTrip(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		violations := generateViolations(size)

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				fs := afero.NewMemMapFs()
				cache, err := NewCache(b.TempDir(), fs)
				if err != nil {
					b.Fatalf("Failed to create cache: %v", err)
				}
				testPath := fmt.Sprintf("test_file_%d.go", i)
				// Create the test file in the filesystem
				_ = afero.WriteFile(fs, testPath, []byte("package test"), 0644)
				b.StartTimer()

				// Write
				err = cache.AddFileWithViolations(testPath, violations)
				if err != nil {
					b.Fatalf("Failed to add violations: %v", err)
				}

				// Read
				_, err = cache.HasEntry(testPath)
				if err != nil {
					b.Fatalf("Failed to read violations: %v", err)
				}
			}

			b.SetBytes(int64(size))
		})
	}
}

// BenchmarkMemoryFootprint measures memory allocations for encoding
func BenchmarkMemoryFootprint(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		violations := wrapViolations(generateViolations(size))

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := marshalLintViolations(violations)
				if err != nil {
					b.Fatalf("Failed to encode: %v", err)
				}
			}
		})
	}
}

// BenchmarkScalability tests performance scaling with very large datasets
func BenchmarkScalability(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		violations := wrapViolations(generateViolations(size))

		b.Run(fmt.Sprintf("Encode_%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := marshalLintViolations(violations)
				if err != nil {
					b.Fatalf("Failed to encode: %v", err)
				}
			}

			b.SetBytes(int64(size))
		})
	}
}
