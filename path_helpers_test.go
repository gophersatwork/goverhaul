package goverhaul

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/afero"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Unix path",
			input:    "/usr/local/bin",
			expected: "/usr/local/bin",
		},
		{
			name:     "Windows path",
			input:    "C:\\Program Files\\App",
			expected: "C:/Program Files/App",
		},
		{
			name:     "Mixed separators",
			input:    "path/to\\file.txt",
			expected: "path/to/file.txt",
		},
		{
			name:     "Empty path",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestJoinPaths(t *testing.T) {
	tests := []struct {
		name     string
		elements []string
		expected string
	}{
		{
			name:     "Join Unix paths",
			elements: []string{"usr", "local", "bin"},
			expected: "usr/local/bin",
		},
		{
			name:     "Join with empty element",
			elements: []string{"path", "", "file.txt"},
			expected: "path/file.txt",
		},
		{
			name:     "Join with absolute path",
			elements: []string{"/root", "dir", "file.txt"},
			expected: "/root/dir/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JoinPaths(tt.elements...)
			if result != tt.expected {
				t.Errorf("JoinPaths(%v) = %q, want %q", tt.elements, result, tt.expected)
			}
		})
	}
}

func TestIsSubPath(t *testing.T) {
	tests := []struct {
		name       string
		parentPath string
		childPath  string
		expected   bool
	}{
		{
			name:       "Direct child",
			parentPath: "parent",
			childPath:  "parent/child",
			expected:   true,
		},
		{
			name:       "Nested child",
			parentPath: "parent",
			childPath:  "parent/child/grandchild",
			expected:   true,
		},
		{
			name:       "Not a child",
			parentPath: "parent",
			childPath:  "other/path",
			expected:   false,
		},
		{
			name:       "Empty parent",
			parentPath: "",
			childPath:  "any/path",
			expected:   true,
		},
		{
			name:       "Same path",
			parentPath: "path/to/dir",
			childPath:  "path/to/dir",
			expected:   true,
		},
		{
			name:       "Path with relative components",
			parentPath: "path/to/dir",
			childPath:  "path/to/dir/../dir/file.txt",
			expected:   true,
		},
		{
			name:       "Path with relative components going up",
			parentPath: "path/to/dir",
			childPath:  "path/to/dir/../../other",
			expected:   false,
		},
		{
			name:       "Current directory as parent",
			parentPath: ".",
			childPath:  "any/path",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSubPath(tt.parentPath, tt.childPath)
			if result != tt.expected {
				t.Errorf("IsSubPath(%q, %q) = %v, want %v", tt.parentPath, tt.childPath, result, tt.expected)
			}
		})
	}
}

func TestAbsPath(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	normalizedTempDir := NormalizePath(tempDir)

	tests := []struct {
		name     string
		input    string
		expected string
		setup    func() string
	}{
		{
			name:  "Relative path",
			input: "file.txt",
			setup: func() string {
				// Get current working directory
				wd, _ := filepath.Abs(".")
				return NormalizePath(filepath.Join(wd, "file.txt"))
			},
		},
		{
			name:     "Already absolute path",
			input:    normalizedTempDir,
			expected: normalizedTempDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var expected string
			if tt.setup != nil {
				expected = tt.setup()
			} else {
				expected = tt.expected
			}

			result := AbsPath(tt.input)
			if result != expected {
				t.Errorf("AbsPath(%q) = %q, want %q", tt.input, result, expected)
			}
		})
	}
}

func TestDirPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Unix path",
			input:    "/usr/local/bin/app",
			expected: "/usr/local/bin",
		},
		{
			name:     "Relative path",
			input:    "dir/file.txt",
			expected: "dir",
		},
		{
			name:     "File in root",
			input:    "/file.txt",
			expected: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DirPath(tt.input)
			if result != tt.expected {
				t.Errorf("DirPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPathExists(t *testing.T) {
	memFs := afero.NewMemMapFs()
	tempDir := "/tmp/cache"
	err := memFs.Mkdir(tempDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	err = afero.WriteFile(memFs, testFilePath, []byte("test content"), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Existing file",
			path:     testFilePath,
			expected: true,
		},
		{
			name:     "Existing directory",
			path:     tempDir,
			expected: true,
		},
		{
			name:     "Non-existent file",
			path:     filepath.Join(tempDir, "nonexistent.txt"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := afero.Exists(memFs, tt.path)
			if err != nil {
				t.Fatalf("Failed to check if path exists: %v", err)
			}
			if exists != tt.expected {
				t.Errorf("PathExists(%q) = %v, want %v", tt.path, exists, tt.expected)
			}
		})
	}
}

func TestIsAbsPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Absolute path",
			path:     "/usr/local/bin",
			expected: true,
		},
		{
			name:     "Relative path",
			path:     "dir/file.txt",
			expected: false,
		},
		{
			name:     "Current directory",
			path:     ".",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAbsPath(tt.path)
			if result != tt.expected {
				t.Errorf("IsAbsPath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestPlatformSpecificBehavior tests platform-specific behavior
// to ensure our helpers work correctly on the current platform
func TestPlatformSpecificBehavior(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Windows-specific tests as we only support Unix paths now")
	} else {
		// Test Unix-specific behavior
		t.Run("Unix path normalization", func(t *testing.T) {
			path := "/home/user/documents"
			normalized := NormalizePath(path)
			if normalized != path {
				t.Errorf("NormalizePath(%q) = %q, want %q", path, normalized, path)
			}
		})
	}
}
