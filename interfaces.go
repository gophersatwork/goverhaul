package goverhaul

// CacheInterface defines the interface for the MUS-based cache implementation.
// The cache uses MUS encoding for high-performance serialization.
type CacheInterface interface {
	// AddFile adds a file entry to the cache without violations
	AddFile(path string) error

	// AddFileWithViolations stores violations for a file using MUS encoding
	AddFileWithViolations(path string, violations []LintViolation) error

	// HasEntry checks if a file has cached violations and returns them
	HasEntry(filePath string) (LintViolations, error)
}