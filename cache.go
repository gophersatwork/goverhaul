package goverhaul

import (
	"encoding/json"
	"errors"

	"github.com/gophersatwork/granular"
	"github.com/spf13/afero"
)

type LintCache struct {
	gCache *granular.Cache
	fs     afero.Fs
}

func NewCache(path string) (*LintCache, error) {
	cache, err := granular.New(path)
	if err != nil {
		return nil, err
	}
	return &LintCache{
		gCache: cache,
	}, nil
}

func NewCacheWithFs(path string, fs afero.Fs) (*LintCache, error) {
	cache, err := granular.New(path, granular.WithFs(fs))
	if err != nil {
		return nil, err
	}
	return &LintCache{
		gCache: cache,
		fs:     fs,
	}, nil
}

func (c *LintCache) AddFile(path string) error {
	// Normalize the path for consistent caching
	normalizedPath := NormalizePath(path)
	key := granular.Key{
		Inputs: []granular.Input{granular.FileInput{
			Path: normalizedPath,
			Fs:   c.fs,
		}},
	}
	err := c.gCache.Store(key, granular.Result{})
	if err != nil {
		return err
	}

	return nil
}

func (c *LintCache) AddFileWithViolations(path string, lv []LintViolation) error {
	// Normalize the path for consistent caching
	normalizedPath := NormalizePath(path)

	key := granular.Key{
		Inputs: []granular.Input{granular.FileInput{
			Path: normalizedPath,
			Fs:   c.fs,
		}},
	}

	metadata := make(map[string]string)
	// Create a LintViolations struct to hold the violations
	violations := LintViolations{
		Violations: lv,
	}
	lvBytes, err := json.Marshal(violations)
	if err != nil {
		return err
	}

	metadata["violations"] = string(lvBytes)
	res := granular.Result{
		Metadata: metadata,
	}

	err = c.gCache.Store(key, res)
	if err != nil {
		return err
	}

	return nil
}

var (
	ErrEntryNotFound           = errors.New("entry not found")
	ErrReadingCachedViolations = errors.New("cached violations are invalid")
)

func (c *LintCache) HasEntry(filePath string) (LintViolations, error) {
	// Normalize the path for consistent caching
	normalizedPath := NormalizePath(filePath)

	key := granular.Key{
		Inputs: []granular.Input{granular.FileInput{
			Path: normalizedPath,
			Fs:   c.fs,
		}},
	}

	result, found, _ := c.gCache.Get(key)
	var err error

	if !found {
		return LintViolations{}, ErrEntryNotFound
	}

	violations, ok := result.Metadata["violations"]
	if !ok {
		return LintViolations{}, nil
	}

	var lv LintViolations
	err = json.Unmarshal([]byte(violations), &lv)
	if err != nil {
		return LintViolations{}, ErrReadingCachedViolations
	}
	return lv, nil
}
