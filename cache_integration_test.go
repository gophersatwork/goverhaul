package goverhaul

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCacheEncoderIntegration tests all three encoder options work correctly
func TestCacheEncoderIntegration(t *testing.T) {
	testCases := []struct {
		name    string
		encoder CacheEncoder
	}{
		{"JSON", EncoderJSON},
		{"Gob", EncoderGob},
		{"MUS", EncoderMUS},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()

			// Create test file
			testFile := "integration_test.go"
			content := `package test
import "fmt"
import "internal/database"
import "unsafe"`
			err := afero.WriteFile(fs, testFile, []byte(content), 0644)
			require.NoError(t, err)

			// Create cache with specified encoder
			cache, err := NewImprovedCache(CacheConfig{
				Path:    fmt.Sprintf(".test_%s.cache", tc.name),
				Encoder: tc.encoder,
				Fs:      fs,
			})
			require.NoError(t, err)

			// Create test violations
			violations := []LintViolation{
				{
					File:    testFile,
					Import:  "internal/database",
					Rule:    "no-db",
					Cause:   "Database imports not allowed",
					Details: "Use repository pattern instead",
				},
				{
					File:    testFile,
					Import:  "unsafe",
					Rule:    "no-unsafe",
					Cause:   "Unsafe package is forbidden",
					Details: "Can cause memory corruption",
				},
			}

			// Store violations
			err = cache.AddFileWithViolations(testFile, violations)
			require.NoError(t, err)

			// Retrieve violations
			result, err := cache.HasEntry(testFile)
			require.NoError(t, err)

			// Verify violations
			assert.Len(t, result.Violations, 2)
			for i, v := range result.Violations {
				assert.True(t, v.Cached, "violation %d should be marked as cached", i)
				assert.Equal(t, violations[i].Import, v.Import)
				assert.Equal(t, violations[i].Rule, v.Rule)
				assert.Equal(t, violations[i].Cause, v.Cause)
				assert.Equal(t, violations[i].Details, v.Details)
			}

			// Check stats
			stats := cache.GetStats()
			expectedEncoder := map[CacheEncoder]string{
				EncoderJSON: "json",
				EncoderGob:  "gob",
				EncoderMUS:  "mus",
			}
			assert.Equal(t, expectedEncoder[tc.encoder], stats.Encoder)
		})
	}
}

// TestCacheEncoderPerformance compares performance of different encoders
func TestCacheEncoderPerformance(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create test file
	testFile := "perf_test.go"
	content := `package test`
	err := afero.WriteFile(fs, testFile, []byte(content), 0644)
	require.NoError(t, err)

	// Generate large set of violations
	violations := make([]LintViolation, 1000)
	for i := range violations {
		violations[i] = LintViolation{
			File:    testFile,
			Import:  fmt.Sprintf("package%d", i),
			Rule:    fmt.Sprintf("rule%d", i%10),
			Cause:   fmt.Sprintf("Violation cause for package %d", i),
			Details: fmt.Sprintf("Detailed explanation for violation %d", i),
		}
	}

	encoders := []struct {
		name    string
		encoder CacheEncoder
	}{
		{"JSON", EncoderJSON},
		{"Gob", EncoderGob},
		{"MUS", EncoderMUS},
	}

	for _, enc := range encoders {
		t.Run(enc.name, func(t *testing.T) {
			cache, err := NewImprovedCache(CacheConfig{
				Path:    fmt.Sprintf(".perf_%s.cache", enc.name),
				Encoder: enc.encoder,
				Fs:      fs,
			})
			require.NoError(t, err)

			// Store violations
			err = cache.AddFileWithViolations(testFile, violations)
			require.NoError(t, err)

			// Retrieve violations
			result, err := cache.HasEntry(testFile)
			require.NoError(t, err)

			// Verify count
			assert.Len(t, result.Violations, 1000)
			assert.True(t, result.Violations[0].Cached)
			assert.True(t, result.Violations[999].Cached)
		})
	}
}

// TestMusCacheHelperFunction tests the NewMusCacheIntegrated helper
func TestMusCacheHelperFunction(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create cache using the helper function
	cache, err := NewMusCacheIntegrated(".mus_helper.cache", fs)
	require.NoError(t, err)
	assert.NotNil(t, cache)

	// Verify it's using MUS encoder
	stats := cache.GetStats()
	assert.Equal(t, "mus", stats.Encoder)

	// Test basic operations
	testFile := "helper_test.go"
	err = afero.WriteFile(fs, testFile, []byte("package test"), 0644)
	require.NoError(t, err)

	violations := []LintViolation{
		{
			File:   testFile,
			Import: "test/package",
			Rule:   "test-rule",
			Cause:  "Test cause",
		},
	}

	err = cache.AddFileWithViolations(testFile, violations)
	require.NoError(t, err)

	result, err := cache.HasEntry(testFile)
	require.NoError(t, err)
	assert.Len(t, result.Violations, 1)
	assert.True(t, result.Violations[0].Cached)
}