package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/gophersatwork/goverhaul"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_HandleInitialize(t *testing.T) {
	fs := afero.NewMemMapFs()
	config := &goverhaul.Config{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	linter, err := goverhaul.NewLinter(*config, logger, fs)
	require.NoError(t, err)

	var buf bytes.Buffer
	server := NewServer(linter, config, fs, logger, &buf, &buf)

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{}`),
	}

	resp := server.handleRequest(context.Background(), req)
	require.NotNil(t, resp)
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 1, resp.ID)
	assert.Nil(t, resp.Error)

	// Check capabilities
	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)

	capabilities, ok := result["capabilities"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, capabilities["hoverProvider"])
}

func TestServer_HandleHover(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create a test file
	testFile := `package api

import (
	"internal/database"
)

func GetUser() {
}
`
	err := afero.WriteFile(fs, "internal/api/handler.go", []byte(testFile), 0644)
	require.NoError(t, err)

	// Create go.mod in project root
	goMod := `module github.com/test/app

go 1.24
`
	err = afero.WriteFile(fs, "go.mod", []byte(goMod), 0644)
	require.NoError(t, err)

	config := &goverhaul.Config{
		Rules: []goverhaul.Rule{
			{
				Path: "internal/api",
				Prohibited: []goverhaul.ProhibitedPkg{
					{
						Name:  "internal/database",
						Cause: "API layer must not access database directly",
					},
				},
			},
		},
		Modfile: "go.mod",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	linter, err := goverhaul.NewLinter(*config, logger, fs)
	require.NoError(t, err)

	var buf bytes.Buffer
	server := NewServer(linter, config, fs, logger, &buf, &buf)

	// Create hover request
	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{
			URI: "file:///internal/api/handler.go",
		},
		Position: Position{
			Line:      3,
			Character: 5,
		},
	}

	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	req := Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "textDocument/hover",
		Params:  paramsJSON,
	}

	resp := server.handleRequest(context.Background(), req)
	require.NotNil(t, resp)
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 2, resp.ID)
	assert.Nil(t, resp.Error)
	require.NotNil(t, resp.Result)

	// Check hover result
	hover, ok := resp.Result.(*Hover)
	require.True(t, ok)
	assert.Equal(t, Markdown, hover.Contents.Kind)
	assert.Contains(t, hover.Contents.Value, "internal/database")
}

func TestServer_HandleHover_NotOnImport(t *testing.T) {
	fs := afero.NewMemMapFs()

	testFile := `package main

import (
	"fmt"
)

func main() {
	fmt.Println("hello")
}
`
	err := afero.WriteFile(fs, "main.go", []byte(testFile), 0644)
	require.NoError(t, err)

	config := &goverhaul.Config{
		Rules: []goverhaul.Rule{},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	linter, err := goverhaul.NewLinter(*config, logger, fs)
	require.NoError(t, err)

	var buf bytes.Buffer
	server := NewServer(linter, config, fs, logger, &buf, &buf)

	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{
			URI: "file:///main.go",
		},
		Position: Position{
			Line:      6,
			Character: 0,
		},
	}

	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err)

	req := Request{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "textDocument/hover",
		Params:  paramsJSON,
	}

	resp := server.handleRequest(context.Background(), req)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
	assert.Nil(t, resp.Result)
}

func TestServer_HandleMethodNotFound(t *testing.T) {
	fs := afero.NewMemMapFs()
	config := &goverhaul.Config{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	linter, err := goverhaul.NewLinter(*config, logger, fs)
	require.NoError(t, err)

	var buf bytes.Buffer
	server := NewServer(linter, config, fs, logger, &buf, &buf)

	req := Request{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "unknownMethod",
		Params:  json.RawMessage(`{}`),
	}

	resp := server.handleRequest(context.Background(), req)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, MethodNotFound, resp.Error.Code)
}

func TestServer_HandleShutdown(t *testing.T) {
	fs := afero.NewMemMapFs()
	config := &goverhaul.Config{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	linter, err := goverhaul.NewLinter(*config, logger, fs)
	require.NoError(t, err)

	var buf bytes.Buffer
	server := NewServer(linter, config, fs, logger, &buf, &buf)

	req := Request{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "shutdown",
		Params:  json.RawMessage(`{}`),
	}

	resp := server.handleRequest(context.Background(), req)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
	assert.Nil(t, resp.Result)
}

func TestServer_StartAndStop(t *testing.T) {
	fs := afero.NewMemMapFs()
	config := &goverhaul.Config{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	linter, err := goverhaul.NewLinter(*config, logger, fs)
	require.NoError(t, err)

	// Create a pipe for communication
	reader := bytes.NewReader([]byte{})
	var writer bytes.Buffer

	server := NewServer(linter, config, fs, logger, reader, &writer)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = server.Start(ctx)
	// Should return context deadline exceeded or nil (if EOF)
	assert.True(t, err == context.DeadlineExceeded || err == nil)
}

func TestServer_InvalidHoverParams(t *testing.T) {
	fs := afero.NewMemMapFs()
	config := &goverhaul.Config{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	linter, err := goverhaul.NewLinter(*config, logger, fs)
	require.NoError(t, err)

	var buf bytes.Buffer
	server := NewServer(linter, config, fs, logger, &buf, &buf)

	req := Request{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "textDocument/hover",
		Params:  json.RawMessage(`invalid json`),
	}

	resp := server.handleRequest(context.Background(), req)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, InvalidParams, resp.Error.Code)
}
