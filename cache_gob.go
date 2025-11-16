package goverhaul

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gophersatwork/granular"
	"github.com/spf13/afero"
)

// CacheEncoder defines the encoding format for cache serialization
type CacheEncoder int

const (
	// EncoderJSON uses JSON encoding (slower but human-readable)
	EncoderJSON CacheEncoder = iota
	// EncoderGob uses Gob encoding (faster, smaller, but binary)
	EncoderGob
	// EncoderMUS uses MUS encoding (fastest, most compact, binary)
	EncoderMUS
)

// CacheConfig holds configuration for the cache
type CacheConfig struct {
	Path    string
	Encoder CacheEncoder
	Fs      afero.Fs
}

// ImprovedLintCache provides high-performance caching with configurable encoding
type ImprovedLintCache struct {
	gCache  *granular.Cache
	fs      afero.Fs
	encoder CacheEncoder
}

// NewImprovedCache creates a new cache with the specified configuration
func NewImprovedCache(cfg CacheConfig) (*ImprovedLintCache, error) {
	opts := []granular.Option{}
	if cfg.Fs != nil {
		opts = append(opts, granular.WithFs(cfg.Fs))
	}

	cache, err := granular.New(cfg.Path, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create granular cache: %w", err)
	}

	return &ImprovedLintCache{
		gCache:  cache,
		fs:      cfg.Fs,
		encoder: cfg.Encoder,
	}, nil
}

// NewGobCache creates a cache with Gob encoding (recommended for performance)
func NewGobCache(path string, fs afero.Fs) (*ImprovedLintCache, error) {
	return NewImprovedCache(CacheConfig{
		Path:    path,
		Encoder: EncoderGob,
		Fs:      fs,
	})
}

// NewMusCacheIntegrated creates a cache with MUS encoding (best performance)
func NewMusCacheIntegrated(path string, fs afero.Fs) (*ImprovedLintCache, error) {
	return NewImprovedCache(CacheConfig{
		Path:    path,
		Encoder: EncoderMUS,
		Fs:      fs,
	})
}

// AddFile adds a file entry to the cache without violations
func (c *ImprovedLintCache) AddFile(path string) error {
	normalizedPath := NormalizePath(path)
	key := granular.Key{
		Inputs: []granular.Input{granular.FileInput{
			Path: normalizedPath,
			Fs:   c.fs,
		}},
	}

	return c.gCache.Store(key, granular.Result{})
}

// AddFileWithViolations stores violations for a file using the configured encoder
func (c *ImprovedLintCache) AddFileWithViolations(path string, lv []LintViolation) error {
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

	// Encode violations based on configured encoder
	encoded, err := c.encodeViolations(violations)
	if err != nil {
		return fmt.Errorf("failed to encode violations: %w", err)
	}

	metadata := map[string]string{
		"violations": encoded,
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
func (c *ImprovedLintCache) HasEntry(filePath string) (LintViolations, error) {
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

	// Decode violations using the configured encoder
	lv, err := c.decodeViolations(violations, c.encoder)
	if err != nil {
		return LintViolations{}, fmt.Errorf("%w: %v", ErrReadingCachedViolations, err)
	}

	// Mark violations as cached
	for i := range lv.Violations {
		lv.Violations[i].Cached = true
	}

	return lv, nil
}

// encodeViolations encodes violations using the configured encoder
func (c *ImprovedLintCache) encodeViolations(v LintViolations) (string, error) {
	switch c.encoder {
	case EncoderGob:
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		if err := enc.Encode(v); err != nil {
			return "", err
		}
		// Convert to string for storage in metadata
		return buf.String(), nil

	case EncoderMUS:
		// Use the MUS encoder from cache_mus.go
		return encodeMusViolations(v)

	case EncoderJSON:
		fallthrough
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}

// decodeViolations decodes violations based on the specified encoder
func (c *ImprovedLintCache) decodeViolations(data string, encoder CacheEncoder) (LintViolations, error) {
	var lv LintViolations

	switch encoder {
	case EncoderGob:
		buf := bytes.NewBufferString(data)
		dec := gob.NewDecoder(buf)
		if err := dec.Decode(&lv); err != nil {
			return LintViolations{}, err
		}

	case EncoderMUS:
		// Use the MUS decoder from cache_mus.go
		return decodeMusViolations(data)

	case EncoderJSON:
		fallthrough
	default:
		if err := json.Unmarshal([]byte(data), &lv); err != nil {
			return LintViolations{}, err
		}
	}

	return lv, nil
}

// Clear removes all entries from the cache
func (c *ImprovedLintCache) Clear() error {
	// This would need to be implemented in the granular package
	// For now, return an error indicating it's not supported
	return errors.New("clear operation not supported by underlying cache")
}

// Stats returns cache statistics
type CacheStats struct {
	Encoder    string
	EntryCount int // Would need granular support
}

// GetStats returns cache statistics
func (c *ImprovedLintCache) GetStats() CacheStats {
	encoderName := "json"
	switch c.encoder {
	case EncoderGob:
		encoderName = "gob"
	case EncoderMUS:
		encoderName = "mus"
	}

	return CacheStats{
		Encoder: encoderName,
		// EntryCount would need support from granular
	}
}

