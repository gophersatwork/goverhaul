package goverhaul

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"testing"
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

// BenchmarkEncoding_MUS_Encode benchmarks MUS encoding performance
func BenchmarkEncoding_MUS_Encode(b *testing.B) {
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

// BenchmarkEncoding_MUS_Decode benchmarks MUS decoding performance
func BenchmarkEncoding_MUS_Decode(b *testing.B) {
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

// BenchmarkEncoding_MUS_RoundTrip benchmarks complete MUS encode/decode cycle
func BenchmarkEncoding_MUS_RoundTrip(b *testing.B) {
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

// BenchmarkEncoding_Gob_Encode benchmarks Gob encoding performance
func BenchmarkEncoding_Gob_Encode(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000, 100000}

	for _, size := range sizes {
		violations := wrapViolations(generateViolations(size))

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			var buf bytes.Buffer
			for i := 0; i < b.N; i++ {
				buf.Reset()
				enc := gob.NewEncoder(&buf)
				err := enc.Encode(violations)
				if err != nil {
					b.Fatalf("Failed to encode: %v", err)
				}
			}

			b.SetBytes(int64(buf.Len()))
			b.ReportMetric(float64(buf.Len()), "bytes/op")
		})
	}
}

// BenchmarkEncoding_Gob_Decode benchmarks Gob decoding performance
func BenchmarkEncoding_Gob_Decode(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000, 100000}

	for _, size := range sizes {
		violations := wrapViolations(generateViolations(size))

		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		err := enc.Encode(violations)
		if err != nil {
			b.Fatalf("Failed to encode: %v", err)
		}
		encoded := buf.Bytes()

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				buf := bytes.NewBuffer(encoded)
				dec := gob.NewDecoder(buf)
				var decoded LintViolations
				err := dec.Decode(&decoded)
				if err != nil {
					b.Fatalf("Failed to decode: %v", err)
				}
			}

			b.SetBytes(int64(len(encoded)))
		})
	}
}

// BenchmarkEncoding_Gob_RoundTrip benchmarks complete Gob encode/decode cycle
func BenchmarkEncoding_Gob_RoundTrip(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000, 100000}

	for _, size := range sizes {
		violations := wrapViolations(generateViolations(size))

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var buf bytes.Buffer
				enc := gob.NewEncoder(&buf)
				err := enc.Encode(violations)
				if err != nil {
					b.Fatalf("Failed to encode: %v", err)
				}

				dec := gob.NewDecoder(&buf)
				var decoded LintViolations
				err = dec.Decode(&decoded)
				if err != nil {
					b.Fatalf("Failed to decode: %v", err)
				}
			}

			b.SetBytes(int64(size))
		})
	}
}

// BenchmarkEncoding_JSON_Encode benchmarks JSON encoding performance
func BenchmarkEncoding_JSON_Encode(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000, 100000}

	for _, size := range sizes {
		violations := wrapViolations(generateViolations(size))

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			var result []byte
			var err error
			for i := 0; i < b.N; i++ {
				result, err = json.Marshal(violations)
				if err != nil {
					b.Fatalf("Failed to encode: %v", err)
				}
			}

			b.SetBytes(int64(len(result)))
			b.ReportMetric(float64(len(result)), "bytes/op")
		})
	}
}

// BenchmarkEncoding_JSON_Decode benchmarks JSON decoding performance
func BenchmarkEncoding_JSON_Decode(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000, 100000}

	for _, size := range sizes {
		violations := wrapViolations(generateViolations(size))
		encoded, err := json.Marshal(violations)
		if err != nil {
			b.Fatalf("Failed to encode: %v", err)
		}

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var decoded LintViolations
				err := json.Unmarshal(encoded, &decoded)
				if err != nil {
					b.Fatalf("Failed to decode: %v", err)
				}
			}

			b.SetBytes(int64(len(encoded)))
		})
	}
}

// BenchmarkEncoding_JSON_RoundTrip benchmarks complete JSON encode/decode cycle
func BenchmarkEncoding_JSON_RoundTrip(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000, 100000}

	for _, size := range sizes {
		violations := wrapViolations(generateViolations(size))

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				encoded, err := json.Marshal(violations)
				if err != nil {
					b.Fatalf("Failed to encode: %v", err)
				}

				var decoded LintViolations
				err = json.Unmarshal(encoded, &decoded)
				if err != nil {
					b.Fatalf("Failed to decode: %v", err)
				}
			}

			b.SetBytes(int64(size))
		})
	}
}

// BenchmarkSerializedSize compares serialized sizes across encoders
func BenchmarkSerializedSize(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000, 100000}

	for _, size := range sizes {
		violations := wrapViolations(generateViolations(size))

		b.Run(fmt.Sprintf("MUS_%d_violations", size), func(b *testing.B) {
			encoded, err := marshalLintViolations(violations)
			if err != nil {
				b.Fatalf("Failed to encode: %v", err)
			}
			b.ReportMetric(float64(len(encoded)), "bytes")
			b.ReportMetric(float64(len(encoded))/float64(size), "bytes/violation")
		})

		b.Run(fmt.Sprintf("Gob_%d_violations", size), func(b *testing.B) {
			var buf bytes.Buffer
			enc := gob.NewEncoder(&buf)
			err := enc.Encode(violations)
			if err != nil {
				b.Fatalf("Failed to encode: %v", err)
			}
			b.ReportMetric(float64(buf.Len()), "bytes")
			b.ReportMetric(float64(buf.Len())/float64(size), "bytes/violation")
		})

		b.Run(fmt.Sprintf("JSON_%d_violations", size), func(b *testing.B) {
			encoded, err := json.Marshal(violations)
			if err != nil {
				b.Fatalf("Failed to encode: %v", err)
			}
			b.ReportMetric(float64(len(encoded)), "bytes")
			b.ReportMetric(float64(len(encoded))/float64(size), "bytes/violation")
		})
	}
}

// BenchmarkCacheWrite_MUS benchmarks cache write operations with MUS encoding
func BenchmarkCacheWrite_MUS(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		violations := generateViolations(size)

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				cache, err := NewMusCache(b.TempDir(), nil)
				if err != nil {
					b.Fatalf("Failed to create cache: %v", err)
				}
				testPath := fmt.Sprintf("test_file_%d.go", i)
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

// BenchmarkCacheWrite_Gob benchmarks cache write operations with Gob encoding
func BenchmarkCacheWrite_Gob(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		violations := generateViolations(size)

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				cache, err := NewGobCache(b.TempDir(), nil)
				if err != nil {
					b.Fatalf("Failed to create cache: %v", err)
				}
				testPath := fmt.Sprintf("test_file_%d.go", i)
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

// BenchmarkCacheRead_MUS benchmarks cache read operations with MUS encoding
func BenchmarkCacheRead_MUS(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		violations := generateViolations(size)

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			// Setup: create cache and populate it
			cache, err := NewMusCache(b.TempDir(), nil)
			if err != nil {
				b.Fatalf("Failed to create cache: %v", err)
			}

			testPath := "test_file.go"
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

// BenchmarkCacheRead_Gob benchmarks cache read operations with Gob encoding
func BenchmarkCacheRead_Gob(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		violations := generateViolations(size)

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			// Setup: create cache and populate it
			cache, err := NewGobCache(b.TempDir(), nil)
			if err != nil {
				b.Fatalf("Failed to create cache: %v", err)
			}

			testPath := "test_file.go"
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

// BenchmarkCacheRoundTrip_MUS benchmarks complete cache write+read cycle with MUS
func BenchmarkCacheRoundTrip_MUS(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		violations := generateViolations(size)

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				cache, err := NewMusCache(b.TempDir(), nil)
				if err != nil {
					b.Fatalf("Failed to create cache: %v", err)
				}
				testPath := fmt.Sprintf("test_file_%d.go", i)
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

// BenchmarkCacheRoundTrip_Gob benchmarks complete cache write+read cycle with Gob
func BenchmarkCacheRoundTrip_Gob(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		violations := generateViolations(size)

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				cache, err := NewGobCache(b.TempDir(), nil)
				if err != nil {
					b.Fatalf("Failed to create cache: %v", err)
				}
				testPath := fmt.Sprintf("test_file_%d.go", i)
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

// BenchmarkMemoryFootprint_MUS measures memory allocations for MUS encoding
func BenchmarkMemoryFootprint_MUS(b *testing.B) {
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

// BenchmarkMemoryFootprint_Gob measures memory allocations for Gob encoding
func BenchmarkMemoryFootprint_Gob(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		violations := wrapViolations(generateViolations(size))

		b.Run(fmt.Sprintf("%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var buf bytes.Buffer
				enc := gob.NewEncoder(&buf)
				err := enc.Encode(violations)
				if err != nil {
					b.Fatalf("Failed to encode: %v", err)
				}
			}
		})
	}
}

// BenchmarkScalability_MUS tests MUS performance scaling with very large datasets
func BenchmarkScalability_MUS(b *testing.B) {
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

// BenchmarkScalability_Gob tests Gob performance scaling with very large datasets
func BenchmarkScalability_Gob(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		violations := wrapViolations(generateViolations(size))

		b.Run(fmt.Sprintf("Encode_%d_violations", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var buf bytes.Buffer
				enc := gob.NewEncoder(&buf)
				err := enc.Encode(violations)
				if err != nil {
					b.Fatalf("Failed to encode: %v", err)
				}
			}

			b.SetBytes(int64(size))
		})
	}
}

// BenchmarkComparison_AllEncoders provides side-by-side comparison
func BenchmarkComparison_AllEncoders(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		violations := wrapViolations(generateViolations(size))

		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.Run("MUS_Encode", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_, _ = marshalLintViolations(violations)
				}
			})

			b.Run("Gob_Encode", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					var buf bytes.Buffer
					enc := gob.NewEncoder(&buf)
					_ = enc.Encode(violations)
				}
			})

			b.Run("JSON_Encode", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_, _ = json.Marshal(violations)
				}
			})
		})
	}
}
