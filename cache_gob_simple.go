package goverhaul

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"

	"github.com/gophersatwork/granular"
	"github.com/spf13/afero"
)

// SimpleGobCache provides high-performance caching using Gob encoding
type SimpleGobCache struct {
	gCache *granular.Cache
	fs     afero.Fs
}

// NewSimpleGobCache creates a new cache with Gob encoding for maximum performance
func NewSimpleGobCache(path string, fs afero.Fs) (*SimpleGobCache, error) {
	opts := []granular.Option{}
	if fs != nil {
		opts = append(opts, granular.WithFs(fs))
	}

	cache, err := granular.New(path, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create granular cache: %w", err)
	}

	return &SimpleGobCache{
		gCache: cache,
		fs:     fs,
	}, nil
}

// AddFile adds a file entry to the cache without violations
func (c *SimpleGobCache) AddFile(path string) error {
	normalizedPath := NormalizePath(path)
	key := granular.Key{
		Inputs: []granular.Input{granular.FileInput{
			Path: normalizedPath,
			Fs:   c.fs,
		}},
	}

	return c.gCache.Store(key, granular.Result{})
}

// AddFileWithViolations stores violations for a file using Gob encoding
func (c *SimpleGobCache) AddFileWithViolations(path string, lv []LintViolation) error {
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

	// Encode violations using Gob
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(violations); err != nil {
		return fmt.Errorf("failed to encode violations: %w", err)
	}

	metadata := map[string]string{
		"violations": buf.String(),
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
func (c *SimpleGobCache) HasEntry(filePath string) (LintViolations, error) {
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

	// Decode violations using Gob
	var lv LintViolations
	buf := bytes.NewBufferString(violations)
	dec := gob.NewDecoder(buf)
	if err := dec.Decode(&lv); err != nil {
		return LintViolations{}, fmt.Errorf("%w: %v", ErrReadingCachedViolations, err)
	}

	// Mark violations as cached
	for i := range lv.Violations {
		lv.Violations[i].Cached = true
	}

	return lv, nil
}

// Clear removes all entries from the cache
func (c *SimpleGobCache) Clear() error {
	// This would need to be implemented in the granular package
	// For now, return an error indicating it's not supported
	return errors.New("clear operation not supported by underlying cache")
}

// GetStats returns cache statistics
func (c *SimpleGobCache) GetStats() string {
	return "Cache using Gob encoding for maximum performance"
}