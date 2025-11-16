package goverhaul

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGobCache(t *testing.T) {
	fs := afero.NewMemMapFs()

	t.Run("create gob cache", func(t *testing.T) {
		cache, err := NewGobCache(".goverhaul_test.cache", fs)
		require.NoError(t, err)
		assert.NotNil(t, cache)
		assert.Equal(t, EncoderGob, cache.encoder)
	})

	t.Run("add and retrieve with gob encoding", func(t *testing.T) {
		cache, err := NewGobCache(".goverhaul_test.cache", fs)
		require.NoError(t, err)

		// Create test file
		testFile := "test.go"
		err = afero.WriteFile(fs, testFile, []byte("package test\n\nimport \"fmt\""), 0644)
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
		}

		// Add violations
		err = cache.AddFileWithViolations(testFile, violations)
		require.NoError(t, err)

		// Retrieve violations
		result, err := cache.HasEntry(testFile)
		require.NoError(t, err)
		assert.Len(t, result.Violations, 2)
		assert.True(t, result.Violations[0].Cached)
		assert.True(t, result.Violations[1].Cached)
		assert.Equal(t, "internal/database", result.Violations[0].Import)
		assert.Equal(t, "unsafe", result.Violations[1].Import)
	})

}

func TestCacheEncodingPerformance(t *testing.T) {
	// Create test violations
	violations := LintViolations{
		Violations: make([]LintViolation, 100),
	}
	for i := 0; i < 100; i++ {
		violations.Violations[i] = LintViolation{
			File:   fmt.Sprintf("file%d.go", i),
			Import: fmt.Sprintf("import%d", i),
			Rule:   fmt.Sprintf("rule%d", i%10),
			Cause:  "Test violation",
		}
	}

	t.Run("JSON encoding", func(t *testing.T) {
		data, err := json.Marshal(violations)
		require.NoError(t, err)

		var decoded LintViolations
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, len(violations.Violations), len(decoded.Violations))
		t.Logf("JSON size: %d bytes", len(data))
	})

	t.Run("Gob encoding", func(t *testing.T) {
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		err := enc.Encode(violations)
		require.NoError(t, err)

		dec := gob.NewDecoder(&buf)
		var decoded LintViolations
		err = dec.Decode(&decoded)
		require.NoError(t, err)

		assert.Equal(t, len(violations.Violations), len(decoded.Violations))
		t.Logf("Gob size: %d bytes", buf.Len())
	})
}

func BenchmarkCacheEncoding(b *testing.B) {
	violations := LintViolations{
		Violations: make([]LintViolation, 100),
	}
	for i := 0; i < 100; i++ {
		violations.Violations[i] = LintViolation{
			File:   fmt.Sprintf("file%d.go", i),
			Import: fmt.Sprintf("import%d", i),
			Rule:   fmt.Sprintf("rule%d", i%10),
			Cause:  "Test violation cause that is somewhat long",
		}
	}

	b.Run("JSON_Encode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := json.Marshal(violations)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("JSON_Decode", func(b *testing.B) {
		data, _ := json.Marshal(violations)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var decoded LintViolations
			err := json.Unmarshal(data, &decoded)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Gob_Encode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			enc := gob.NewEncoder(&buf)
			err := enc.Encode(violations)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Gob_Decode", func(b *testing.B) {
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		_ = enc.Encode(violations)
		data := buf.Bytes()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			reader := bytes.NewReader(data)
			dec := gob.NewDecoder(reader)
			var decoded LintViolations
			err := dec.Decode(&decoded)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("JSON_RoundTrip", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			data, _ := json.Marshal(violations)
			var decoded LintViolations
			_ = json.Unmarshal(data, &decoded)
		}
	})

	b.Run("Gob_RoundTrip", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			enc := gob.NewEncoder(&buf)
			_ = enc.Encode(violations)

			dec := gob.NewDecoder(&buf)
			var decoded LintViolations
			_ = dec.Decode(&decoded)
		}
	})
}

func TestImprovedCacheStats(t *testing.T) {
	fs := afero.NewMemMapFs()

	cache, err := NewGobCache(".stats_test.cache", fs)
	require.NoError(t, err)

	stats := cache.GetStats()
	assert.Equal(t, "gob", stats.Encoder)
}