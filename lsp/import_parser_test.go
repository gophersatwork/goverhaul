package lsp

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractImportPath(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "extracts import from standard message",
			message:  `import "internal/database" violates rule for path "internal/api"`,
			expected: "internal/database",
		},
		{
			name:     "extracts import with cause",
			message:  `import "fmt" violates rule: Use logging`,
			expected: "fmt",
		},
		{
			name:     "extracts from complex message",
			message:  `File: /test/main.go - import "github.com/pkg/errors" is not allowed`,
			expected: "github.com/pkg/errors",
		},
		{
			name:     "returns empty for no import",
			message:  "some error without import",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractImportPath(tt.message)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetImportLineRange(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		importPath string
		expectErr  bool
		checkRange func(*testing.T, Range)
	}{
		{
			name: "finds single import",
			content: `package main

import "fmt"

func main() {}
`,
			importPath: "fmt",
			expectErr:  false,
			checkRange: func(t *testing.T, r Range) {
				assert.Equal(t, 2, r.Start.Line)
				assert.Greater(t, r.End.Character, r.Start.Character)
			},
		},
		{
			name: "finds import in group",
			content: `package main

import (
	"fmt"
	"strings"
)

func main() {}
`,
			importPath: "strings",
			expectErr:  false,
			checkRange: func(t *testing.T, r Range) {
				assert.Equal(t, 4, r.Start.Line)
			},
		},
		{
			name: "handles aliased import",
			content: `package main

import (
	f "fmt"
)

func main() {}
`,
			importPath: "fmt",
			expectErr:  false,
			checkRange: func(t *testing.T, r Range) {
				assert.Equal(t, 3, r.Start.Line)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			afero.WriteFile(fs, "/test/main.go", []byte(tt.content), 0644)

			rng, err := GetImportLineRange(fs, "/test/main.go", tt.importPath)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkRange != nil {
					tt.checkRange(t, rng)
				}
			}
		})
	}
}

func TestGetImportBlockRange(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		importPath      string
		expectSingle    bool
		expectErr       bool
		checkRange      func(*testing.T, Range)
	}{
		{
			name: "single import includes import keyword",
			content: `package main

import "fmt"

func main() {}
`,
			importPath:   "fmt",
			expectSingle: true,
			expectErr:    false,
			checkRange: func(t *testing.T, r Range) {
				// Should include entire import statement
				assert.Equal(t, 2, r.Start.Line)
				assert.Equal(t, 0, r.Start.Character)
			},
		},
		{
			name: "grouped import only includes import line",
			content: `package main

import (
	"fmt"
	"strings"
)

func main() {}
`,
			importPath:   "fmt",
			expectSingle: false,
			expectErr:    false,
			checkRange: func(t *testing.T, r Range) {
				// Should only include the import line, not the whole block
				assert.Equal(t, 3, r.Start.Line)
				assert.Equal(t, 0, r.Start.Character)
			},
		},
		{
			name: "handles multiple import blocks",
			content: `package main

import "fmt"

import (
	"strings"
	"os"
)

func main() {}
`,
			importPath:   "strings",
			expectSingle: false,
			expectErr:    false,
			checkRange: func(t *testing.T, r Range) {
				assert.Equal(t, 5, r.Start.Line)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			afero.WriteFile(fs, "/test/main.go", []byte(tt.content), 0644)

			rng, isSingle, err := GetImportBlockRange(fs, "/test/main.go", tt.importPath)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectSingle, isSingle)
				if tt.checkRange != nil {
					tt.checkRange(t, rng)
				}
			}
		})
	}
}

func TestGetLineAboveImport(t *testing.T) {
	fs := afero.NewMemMapFs()

	content := `package main

import (
	"fmt"
	"strings"
)

func main() {}
`
	afero.WriteFile(fs, "/test/main.go", []byte(content), 0644)

	line, err := GetLineAboveImport(fs, "/test/main.go", "fmt")
	require.NoError(t, err)

	// The import "fmt" is on line 3 (0-indexed), so line above should be 3
	// (which will be the position where we insert the comment)
	assert.Equal(t, 3, line)
}
