package goverhaul

import (
	"errors"
	"fmt"

	"github.com/gophersatwork/granular"
	"github.com/mus-format/mus-go/ord"
	"github.com/mus-format/mus-go/varint"
	"github.com/spf13/afero"
)

// MusCache provides high-performance caching using MUS serialization
// MUS is a binary serialization format optimized for speed and size
// This implementation uses varint encoding and ord package for optimal performance
type MusCache struct {
	gCache *granular.Cache
	fs     afero.Fs
}

// NewMusCache creates a new cache with MUS encoding for maximum performance
// MUS uses varint encoding and manual serialization for optimal efficiency
func NewMusCache(path string, fs afero.Fs) (*MusCache, error) {
	opts := []granular.Option{}
	if fs != nil {
		opts = append(opts, granular.WithFs(fs))
	}

	cache, err := granular.New(path, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create granular cache: %w", err)
	}

	return &MusCache{
		gCache: cache,
		fs:     fs,
	}, nil
}

// AddFile adds a file entry to the cache without violations
func (c *MusCache) AddFile(path string) error {
	normalizedPath := NormalizePath(path)
	key := granular.Key{
		Inputs: []granular.Input{granular.FileInput{
			Path: normalizedPath,
			Fs:   c.fs,
		}},
	}

	return c.gCache.Store(key, granular.Result{})
}

// AddFileWithViolations stores violations for a file using MUS encoding
func (c *MusCache) AddFileWithViolations(path string, lv []LintViolation) error {
	normalizedPath := NormalizePath(path)

	key := granular.Key{
		Inputs: []granular.Input{granular.FileInput{
			Path: normalizedPath,
			Fs:   c.fs,
		}},
	}

	violations := LintViolations{
		Violations: lv,
	}

	// Encode violations using MUS with varint encoding
	data, err := marshalLintViolations(violations)
	if err != nil {
		return fmt.Errorf("failed to encode violations: %w", err)
	}

	metadata := map[string]string{
		"violations": string(data),
	}

	res := granular.Result{
		Metadata: metadata,
	}

	if err := c.gCache.Store(key, res); err != nil {
		return fmt.Errorf("failed to store in cache: %w", err)
	}

	return nil
}

// HasEntry checks if a file has cached violations and returns them
func (c *MusCache) HasEntry(filePath string) (LintViolations, error) {
	normalizedPath := NormalizePath(filePath)

	key := granular.Key{
		Inputs: []granular.Input{granular.FileInput{
			Path: normalizedPath,
			Fs:   c.fs,
		}},
	}

	result, found, _ := c.gCache.Get(key)
	if !found {
		return LintViolations{}, ErrEntryNotFound
	}

	violations, ok := result.Metadata["violations"]
	if !ok {
		return LintViolations{}, nil
	}

	// Decode violations using MUS
	lv, err := unmarshalLintViolations([]byte(violations))
	if err != nil {
		return LintViolations{}, fmt.Errorf("%w: %v", ErrReadingCachedViolations, err)
	}

	// Mark violations as cached
	for i := range lv.Violations {
		lv.Violations[i].Cached = true
	}

	return lv, nil
}

// Clear removes all entries from the cache
func (c *MusCache) Clear() error {
	// This would need to be implemented in the granular package
	// For now, return an error indicating it's not supported
	return errors.New("clear operation not supported by underlying cache")
}

// GetStats returns cache statistics
func (c *MusCache) GetStats() string {
	return "Cache using MUS encoding with varint for maximum performance and minimal size"
}

// encodeMusViolations encodes LintViolations to string for storage
// This is a wrapper around marshalLintViolations for backward compatibility
func encodeMusViolations(v LintViolations) (string, error) {
	data, err := marshalLintViolations(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// decodeMusViolations decodes LintViolations from string
// This is a wrapper around unmarshalLintViolations for backward compatibility
func decodeMusViolations(data string) (LintViolations, error) {
	return unmarshalLintViolations([]byte(data))
}

// musLintViolationsSize is an alias for lintViolationsSize for backward compatibility
func musLintViolationsSize(lv LintViolations) int {
	return lintViolationsSize(lv)
}

// marshalLintViolations serializes LintViolations using MUS format with varint encoding
func marshalLintViolations(lv LintViolations) ([]byte, error) {
	// Pre-calculate size for efficient allocation
	size := lintViolationsSize(lv)
	buf := make([]byte, size)

	n := marshalLintViolationsTo(lv, buf)
	return buf[:n], nil
}

// unmarshalLintViolations deserializes LintViolations from MUS format
func unmarshalLintViolations(data []byte) (LintViolations, error) {
	lv, _, err := unmarshalLintViolationsFrom(data)
	return lv, err
}

// lintViolationsSize calculates the exact size needed for MUS encoding
func lintViolationsSize(lv LintViolations) int {
	// Size of the violations slice length (varint encoded)
	size := varint.Uint64.Size(uint64(len(lv.Violations)))

	// Size of each violation
	for _, v := range lv.Violations {
		size += lintViolationSize(v)
	}

	return size
}

// lintViolationSize calculates the size needed for a single LintViolation
// Uses ord.SizeString with varint encoding for optimal space efficiency
func lintViolationSize(v LintViolation) int {
	size := 0
	size += ord.SizeString(v.File, varint.PositiveInt)
	size += ord.SizeString(v.Import, varint.PositiveInt)
	size += ord.SizeString(v.Rule, varint.PositiveInt)
	size += ord.SizeString(v.Cause, varint.PositiveInt)
	size += ord.SizeString(v.Details, varint.PositiveInt)
	size += ord.Bool.Size(v.Cached)
	return size
}

// marshalLintViolationsTo serializes LintViolations into the provided buffer
func marshalLintViolationsTo(lv LintViolations, buf []byte) int {
	// Marshal the number of violations using varint
	n := varint.Uint64.Marshal(uint64(len(lv.Violations)), buf)

	// Marshal each violation
	for _, v := range lv.Violations {
		n += marshalLintViolationTo(v, buf[n:])
	}

	return n
}

// marshalLintViolationTo serializes a single LintViolation into the buffer
// Uses ord.MarshalString with varint for length encoding
func marshalLintViolationTo(v LintViolation, buf []byte) int {
	n := ord.MarshalString(v.File, varint.PositiveInt, buf)
	n += ord.MarshalString(v.Import, varint.PositiveInt, buf[n:])
	n += ord.MarshalString(v.Rule, varint.PositiveInt, buf[n:])
	n += ord.MarshalString(v.Cause, varint.PositiveInt, buf[n:])
	n += ord.MarshalString(v.Details, varint.PositiveInt, buf[n:])
	n += ord.Bool.Marshal(v.Cached, buf[n:])
	return n
}

// unmarshalLintViolationsFrom deserializes LintViolations from the buffer
func unmarshalLintViolationsFrom(buf []byte) (LintViolations, int, error) {
	var lv LintViolations

	// Unmarshal the number of violations using varint
	length, n, err := varint.Uint64.Unmarshal(buf)
	if err != nil {
		return lv, n, fmt.Errorf("failed to unmarshal violations length: %w", err)
	}

	// Pre-allocate slice with exact capacity for efficiency
	lv.Violations = make([]LintViolation, length)

	// Unmarshal each violation
	for i := uint64(0); i < length; i++ {
		v, bytesRead, err := unmarshalLintViolationFrom(buf[n:])
		if err != nil {
			return lv, n, fmt.Errorf("failed to unmarshal violation at index %d: %w", i, err)
		}
		lv.Violations[i] = v
		n += bytesRead
	}

	return lv, n, nil
}

// unmarshalLintViolationFrom deserializes a single LintViolation from the buffer
// Manual unmarshaling matching the marshal format
func unmarshalLintViolationFrom(buf []byte) (LintViolation, int, error) {
	var v LintViolation
	var n int

	// Helper function to unmarshal a string with varint length
	unmarshalString := func(data []byte) (string, int, error) {
		// Read length as varint
		length, bytesRead, err := varint.PositiveInt.Unmarshal(data)
		if err != nil {
			return "", 0, fmt.Errorf("failed to read string length: %w", err)
		}

		// Read string bytes
		if len(data[bytesRead:]) < length {
			return "", bytesRead, fmt.Errorf("buffer too small for string of length %d", length)
		}

		str := string(data[bytesRead : bytesRead+length])
		return str, bytesRead + length, nil
	}

	// Unmarshal File
	var m int
	var err error
	v.File, m, err = unmarshalString(buf[n:])
	if err != nil {
		return v, n, fmt.Errorf("failed to unmarshal File: %w", err)
	}
	n += m

	// Unmarshal Import
	v.Import, m, err = unmarshalString(buf[n:])
	if err != nil {
		return v, n, fmt.Errorf("failed to unmarshal Import: %w", err)
	}
	n += m

	// Unmarshal Rule
	v.Rule, m, err = unmarshalString(buf[n:])
	if err != nil {
		return v, n, fmt.Errorf("failed to unmarshal Rule: %w", err)
	}
	n += m

	// Unmarshal Cause
	v.Cause, m, err = unmarshalString(buf[n:])
	if err != nil {
		return v, n, fmt.Errorf("failed to unmarshal Cause: %w", err)
	}
	n += m

	// Unmarshal Details
	v.Details, m, err = unmarshalString(buf[n:])
	if err != nil {
		return v, n, fmt.Errorf("failed to unmarshal Details: %w", err)
	}
	n += m

	// Unmarshal Cached
	v.Cached, m, err = ord.Bool.Unmarshal(buf[n:])
	if err != nil {
		return v, n, fmt.Errorf("failed to unmarshal Cached: %w", err)
	}
	n += m

	return v, n, nil
}
