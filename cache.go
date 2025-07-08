package main

import (
	"encoding/json"
	"errors"

	"github.com/gophersatwork/granular"
)

type LintCache struct {
	gCache *granular.Cache
}

func NewCache(path string) (*granular.Cache, error) {
	cache, err := granular.New(path)
	if err != nil {
		return cache, err
	}

	return cache, nil
}

func (c *LintCache) AddFile(path string) error {
	key := granular.Key{
		Inputs: []granular.Input{granular.FileInput{
			Path: path,
		}},
	}
	err := c.gCache.Store(key, granular.Result{})
	if err != nil {
		return err
	}

	return nil
}

func (c *LintCache) AddFileWithViolations(path string, lv []LintViolation) error {
	key := granular.Key{
		Inputs: []granular.Input{granular.FileInput{
			Path: path,
		}},
	}

	metadata := make(map[string]string)
	lvBytes, err := json.Marshal(lv)
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

var ErrEntryNotFound = errors.New("entry not found")
var ErrReadingCachedViolations = errors.New("cached violations are invalid")

func (c *LintCache) hasEntry(filePath string) (LintViolations, error) {
	key := granular.Key{
		Inputs: []granular.Input{granular.FileInput{
			Path: filePath,
		}},
	}

	result, found, err := c.gCache.Get(key)

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