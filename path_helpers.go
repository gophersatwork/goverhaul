package goverhaul

import (
	"path/filepath"
	"strings"
)

// NormalizePath converts a path to use forward slashes consistently
// regardless of the operating system and cleans the path.
// It removes redundant separators, dot-segments, and normalizes separators to forward slashes.
// Empty paths remain empty to maintain backward compatibility.
func NormalizePath(path string) string {
	// Special case: empty path should remain empty
	if path == "" {
		return ""
	}

	// Clean the path to handle dot-segments and redundant separators
	cleaned := filepath.Clean(path)
	// Replace all backslashes with forward slashes
	return strings.ReplaceAll(cleaned, "\\", "/")
}

// JoinPaths joins path elements and normalizes the result.
// It joins elements using the system's path separator, then normalizes to forward slashes,
// and cleans the path to remove redundant separators and dot-segments.
func JoinPaths(elem ...string) string {
	return NormalizePath(filepath.Join(elem...))
}

// IsSubPath checks if childPath is a subdirectory of parentPath.
// Both paths are normalized before comparison.
func IsSubPath(parentPath, childPath string) bool {
	normalizedParent := NormalizePath(parentPath)
	normalizedChild := NormalizePath(childPath)

	// Handle empty paths
	if normalizedParent == "" || normalizedParent == "." {
		return true // Empty parent means any path is a subpath
	}

	// Check for exact match first
	if normalizedParent == normalizedChild {
		return true
	}

	// Ensure paths end with slash for proper prefix matching
	if !strings.HasSuffix(normalizedParent, "/") {
		normalizedParent += "/"
	}

	return strings.HasPrefix(normalizedChild, normalizedParent)
}

// IsAbsPath checks if a path is absolute
func IsAbsPath(path string) bool {
	return filepath.IsAbs(path)
}

// AbsPath returns the absolute path for a given path
// If an error occurs, it returns the original path
func AbsPath(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return NormalizePath(absPath)
}

// DirPath returns the directory portion of a path
// This function is optimized for Unix paths only
func DirPath(path string) string {
	// Normalize the path first to ensure consistent handling
	normalizedPath := NormalizePath(path)

	// Use filepath.Dir and normalize the result
	return NormalizePath(filepath.Dir(normalizedPath))
}
