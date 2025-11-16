package lsp

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/gophersatwork/goverhaul"
	"github.com/sourcegraph/go-lsp"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	server := NewServer(ctx, logger)

	assert.NotNil(t, server)
	assert.NotNil(t, server.logger)
	assert.NotNil(t, server.fs)
	assert.NotNil(t, server.fileCache)
}

func TestInitialize(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	server := NewServer(ctx, logger)

	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	params := lsp.InitializeParams{
		RootURI: lsp.DocumentURI("file://" + tmpDir),
	}

	result, err := server.Initialize(params)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Capabilities.TextDocumentSync)
	assert.Equal(t, tmpDir, server.rootPath)
}

func TestLoadConfig(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	server := NewServer(ctx, logger)

	// Create a temporary workspace with config
	tmpDir := t.TempDir()
	server.rootPath = tmpDir

	// Use in-memory filesystem for testing
	server.fs = afero.NewMemMapFs()

	// Create a test config file
	configContent := `
rules:
  - path: internal/api
    prohibited:
      - name: internal/database
        cause: API layer must not access database
        severity: error
modfile: go.mod
`
	err := afero.WriteFile(server.fs, filepath.Join(tmpDir, ".goverhaul.yml"), []byte(configContent), 0o644)
	require.NoError(t, err)

	// Load config
	err = server.loadConfig()
	assert.NoError(t, err)
	assert.NotNil(t, server.config)
	assert.NotNil(t, server.linter)
	assert.Len(t, server.config.Rules, 1)
}

func TestLoadConfigNotFound(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	server := NewServer(ctx, logger)

	tmpDir := t.TempDir()
	server.rootPath = tmpDir
	server.fs = afero.NewMemMapFs()

	err := server.loadConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no configuration file found")
}

func TestURIToPath(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{
			name: "simple path",
			uri:  "file:///home/user/project/file.go",
			want: "/home/user/project/file.go",
		},
		{
			name: "path with spaces",
			uri:  "file:///home/user/my%20project/file.go",
			want: "/home/user/my project/file.go",
		},
		{
			name: "windows path",
			uri:  "file:///C:/Users/user/project/file.go",
			want: "/C:/Users/user/project/file.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uriToPath(tt.uri)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPathToURI(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "simple path",
			path: "/home/user/project/file.go",
			want: "file:///home/user/project/file.go",
		},
		{
			name: "path with spaces",
			path: "/home/user/my project/file.go",
			want: "file:///home/user/my%20project/file.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathToURI(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAnalyzeFile(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	server := NewServer(ctx, logger)

	// Create a temporary workspace
	tmpDir := t.TempDir()
	server.rootPath = tmpDir
	server.fs = afero.NewMemMapFs()

	// Create a test config file
	configContent := `
rules:
  - path: internal/api
    prohibited:
      - name: internal/database
        cause: API layer must not access database
        severity: error
modfile: go.mod
`
	err := afero.WriteFile(server.fs, filepath.Join(tmpDir, ".goverhaul.yml"), []byte(configContent), 0o644)
	require.NoError(t, err)

	// Create a test Go file with violations
	testGoFile := filepath.Join(tmpDir, "internal", "api", "handler.go")
	err = server.fs.MkdirAll(filepath.Dir(testGoFile), 0o755)
	require.NoError(t, err)

	goContent := `package api

import (
	"internal/database"
)

func Handler() {
	// use database
}
`
	err = afero.WriteFile(server.fs, testGoFile, []byte(goContent), 0o644)
	require.NoError(t, err)

	// Create go.mod
	goModContent := `module example.com/test

go 1.24
`
	err = afero.WriteFile(server.fs, filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0o644)
	require.NoError(t, err)

	// Load config first
	err = server.loadConfig()
	require.NoError(t, err)

	// Note: We can't fully test analyzeFile without a connection,
	// but we can test that it doesn't panic and that cache is updated
	server.mu.Lock()
	initialCacheSize := len(server.fileCache)
	server.mu.Unlock()

	// This will fail when trying to publish diagnostics, but that's OK for this test
	_ = server.analyzeFile(testGoFile)

	// Verify cache was updated
	server.mu.RLock()
	assert.Greater(t, len(server.fileCache), initialCacheSize)
	server.mu.RUnlock()
}

func TestTextDocumentDidOpen(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	server := NewServer(ctx, logger)

	// Setup
	tmpDir := t.TempDir()
	server.rootPath = tmpDir
	server.fs = afero.NewMemMapFs()

	// Create config
	configContent := `
rules:
  - path: .
    prohibited:
      - name: fmt
        cause: Use structured logging
modfile: go.mod
`
	err := afero.WriteFile(server.fs, filepath.Join(tmpDir, ".goverhaul.yml"), []byte(configContent), 0o644)
	require.NoError(t, err)

	// Create go.mod
	err = afero.WriteFile(server.fs, filepath.Join(tmpDir, "go.mod"), []byte("module test\n"), 0o644)
	require.NoError(t, err)

	testFile := filepath.Join(tmpDir, "test.go")
	err = afero.WriteFile(server.fs, testFile, []byte("package main\n\nimport \"fmt\"\n"), 0o644)
	require.NoError(t, err)

	params := lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI: lsp.DocumentURI("file://" + testFile),
		},
	}

	// This will fail when trying to publish diagnostics, but shouldn't panic
	_ = server.TextDocumentDidOpen(params)
}

func TestTextDocumentDidOpenNonGoFile(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	server := NewServer(ctx, logger)

	params := lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI: lsp.DocumentURI("file:///test.txt"),
		},
	}

	// Should not error for non-Go files
	err := server.TextDocumentDidOpen(params)
	assert.NoError(t, err)
}

func TestShutdown(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	server := NewServer(ctx, logger)

	err := server.Shutdown()
	assert.NoError(t, err)
}

// Benchmark server initialization
func BenchmarkInitialize(b *testing.B) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	tmpDir := b.TempDir()

	params := lsp.InitializeParams{
		RootURI: lsp.DocumentURI("file://" + tmpDir),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server := NewServer(ctx, logger)
		_, _ = server.Initialize(params)
	}
}

// Benchmark diagnostic conversion
func BenchmarkViolationToDiagnostic(b *testing.B) {
	violation := goverhaul.LintViolation{
		File:     "test.go",
		Import:   "internal/database",
		Rule:     "api-no-db",
		Cause:    "API layer must not access database",
		Details:  "This import is explicitly prohibited",
		Severity: goverhaul.SeverityError,
		Position: &goverhaul.Position{
			Line:      5,
			Column:    1,
			EndLine:   5,
			EndColumn: 25,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ViolationToDiagnostic(violation)
	}
}
