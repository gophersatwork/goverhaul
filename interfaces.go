package goverhaul

// CacheInterface defines the common interface for all cache implementations
type CacheInterface interface {
	// AddFile adds a file entry to the cache without violations
	AddFile(path string) error

	// AddFileWithViolations stores violations for a file
	AddFileWithViolations(path string, violations []LintViolation) error

	// HasEntry checks if a file has cached violations and returns them
	HasEntry(filePath string) (LintViolations, error)
}