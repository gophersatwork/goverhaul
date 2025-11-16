package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gophersatwork/goverhaul"
	"github.com/sourcegraph/go-lsp"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/spf13/afero"
)

// Server implements the LSP server for goverhaul
type Server struct {
	conn      *jsonrpc2.Conn
	logger    *slog.Logger
	fs        afero.Fs
	rootURI   string
	rootPath  string
	config    *goverhaul.Config
	linter    *goverhaul.Goverhaul
	fileCache map[string][]goverhaul.LintViolation
	mu        sync.RWMutex
	ctx       context.Context
}

// NewServer creates a new LSP server instance
func NewServer(ctx context.Context, logger *slog.Logger) *Server {
	if logger == nil {
		// Log to stderr for LSP (stdout is reserved for JSON-RPC)
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	return &Server{
		logger:    logger,
		fs:        afero.NewOsFs(),
		fileCache: make(map[string][]goverhaul.LintViolation),
		ctx:       ctx,
	}
}

// Initialize handles the LSP initialize request
func (s *Server) Initialize(params lsp.InitializeParams) (*lsp.InitializeResult, error) {
	s.logger.Info("Initializing goverhaul LSP server", "rootURI", params.RootURI)

	s.rootURI = string(params.RootURI)
	s.rootPath = uriToPath(s.rootURI)

	// Load configuration
	if err := s.loadConfig(); err != nil {
		s.logger.Error("Failed to load configuration", "error", err)
		// Don't fail initialization - we'll try again on file operations
	}

	// Initialize result with server capabilities
	result := &lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync: &lsp.TextDocumentSyncOptionsOrKind{
				Options: &lsp.TextDocumentSyncOptions{
					OpenClose: true,
					Change:    lsp.TDSKIncremental, // Support incremental changes
					Save: &lsp.SaveOptions{
						IncludeText: false,
					},
				},
			},
			// We could add more capabilities here in the future:
			// - CodeActionProvider for quick fixes
			// - HoverProvider for rule documentation
			// - DefinitionProvider for jumping to rule definitions
		},
	}

	s.logger.Info("Server initialized successfully")
	return result, nil
}

// loadConfig loads the goverhaul configuration from the workspace
func (s *Server) loadConfig() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Try to find .goverhaul.yml in the workspace
	configPath := filepath.Join(s.rootPath, ".goverhaul.yml")
	if _, err := s.fs.Stat(configPath); os.IsNotExist(err) {
		// Try config.yml as fallback
		configPath = filepath.Join(s.rootPath, "config.yml")
		if _, err := s.fs.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("no configuration file found in workspace root")
		}
	}

	s.logger.Info("Loading configuration", "path", configPath)

	config, err := goverhaul.LoadConfig(s.fs, s.rootPath, configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	s.config = &config

	// Create linter instance
	linter, err := goverhaul.NewLinter(config, s.logger, s.fs)
	if err != nil {
		return fmt.Errorf("failed to create linter: %w", err)
	}

	s.linter = linter
	s.logger.Info("Configuration loaded successfully", "rules", len(config.Rules))

	return nil
}

// TextDocumentDidOpen handles file open events
func (s *Server) TextDocumentDidOpen(params lsp.DidOpenTextDocumentParams) error {
	uri := string(params.TextDocument.URI)
	filePath := uriToPath(uri)

	s.logger.Info("Document opened", "uri", uri, "path", filePath)

	// Only analyze Go files
	if !strings.HasSuffix(filePath, ".go") {
		return nil
	}

	return s.analyzeFile(filePath)
}

// TextDocumentDidChange handles file change events
func (s *Server) TextDocumentDidChange(params lsp.DidChangeTextDocumentParams) error {
	uri := string(params.TextDocument.URI)
	filePath := uriToPath(uri)

	s.logger.Debug("Document changed", "uri", uri, "path", filePath)

	// We could implement incremental analysis here, but for now we'll
	// wait for save events to avoid too many re-analyses
	return nil
}

// TextDocumentDidSave handles file save events
func (s *Server) TextDocumentDidSave(params lsp.DidSaveTextDocumentParams) error {
	uri := string(params.TextDocument.URI)
	filePath := uriToPath(uri)

	s.logger.Info("Document saved", "uri", uri, "path", filePath)

	// Re-analyze the file on save
	if strings.HasSuffix(filePath, ".go") {
		return s.analyzeFile(filePath)
	}

	// If config file changed, reload configuration
	if strings.HasSuffix(filePath, ".goverhaul.yml") || strings.HasSuffix(filePath, "config.yml") {
		s.logger.Info("Configuration file changed, reloading")
		if err := s.loadConfig(); err != nil {
			s.logger.Error("Failed to reload configuration", "error", err)
			return err
		}
		// Re-analyze all open files
		return s.reanalyzeAllFiles()
	}

	return nil
}

// analyzeFile analyzes a single file and publishes diagnostics
func (s *Server) analyzeFile(filePath string) error {
	// Ensure linter is initialized
	if s.linter == nil {
		if err := s.loadConfig(); err != nil {
			s.logger.Error("Cannot analyze file without configuration", "error", err)
			return err
		}
	}

	s.logger.Debug("Analyzing file", "path", filePath)

	// Run linter on the file
	violations, err := s.linter.Lint(filePath)
	if err != nil {
		s.logger.Error("Failed to analyze file", "path", filePath, "error", err)
		return err
	}

	// Convert violations to diagnostics
	diagnostics := ViolationsToDiagnostics(violations.Violations)

	// Cache the violations
	s.mu.Lock()
	s.fileCache[filePath] = violations.Violations
	s.mu.Unlock()

	// Publish diagnostics
	return s.publishDiagnostics(filePath, diagnostics)
}

// publishDiagnostics sends diagnostics to the client
func (s *Server) publishDiagnostics(filePath string, diagnostics []lsp.Diagnostic) error {
	uri := pathToURI(filePath)

	params := lsp.PublishDiagnosticsParams{
		URI:         lsp.DocumentURI(uri),
		Diagnostics: diagnostics,
	}

	s.logger.Info("Publishing diagnostics", "uri", uri, "count", len(diagnostics))

	// Skip if no connection (e.g., during testing)
	if s.conn == nil {
		s.logger.Debug("Skipping publish diagnostics - no connection")
		return nil
	}

	return s.conn.Notify(s.ctx, "textDocument/publishDiagnostics", params)
}

// reanalyzeAllFiles re-analyzes all cached files
func (s *Server) reanalyzeAllFiles() error {
	s.mu.RLock()
	files := make([]string, 0, len(s.fileCache))
	for file := range s.fileCache {
		files = append(files, file)
	}
	s.mu.RUnlock()

	for _, file := range files {
		if err := s.analyzeFile(file); err != nil {
			s.logger.Error("Failed to re-analyze file", "path", file, "error", err)
		}
	}

	return nil
}

// Shutdown handles the LSP shutdown request
func (s *Server) Shutdown() error {
	s.logger.Info("Shutting down goverhaul LSP server")
	return nil
}

// Handle processes incoming LSP requests
func (s *Server) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (result interface{}, err error) {
	s.conn = conn

	s.logger.Debug("Handling request", "method", req.Method)

	switch req.Method {
	case "initialize":
		var params lsp.InitializeParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, err
		}
		return s.Initialize(params)

	case "initialized":
		// Client has finished initialization
		s.logger.Info("Client initialized")
		return nil, nil

	case "textDocument/didOpen":
		var params lsp.DidOpenTextDocumentParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, err
		}
		return nil, s.TextDocumentDidOpen(params)

	case "textDocument/didChange":
		var params lsp.DidChangeTextDocumentParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, err
		}
		return nil, s.TextDocumentDidChange(params)

	case "textDocument/didSave":
		var params lsp.DidSaveTextDocumentParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, err
		}
		return nil, s.TextDocumentDidSave(params)

	case "shutdown":
		return nil, s.Shutdown()

	case "exit":
		os.Exit(0)
		return nil, nil

	default:
		return nil, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeMethodNotFound,
			Message: fmt.Sprintf("method not found: %s", req.Method),
		}
	}
}

// uriToPath converts a file:// URI to a file system path
func uriToPath(uri string) string {
	// Remove file:// prefix
	path := strings.TrimPrefix(uri, "file://")

	// URL decode (handle %20, etc.)
	// Simple implementation - a full URL decode would be better
	path = strings.ReplaceAll(path, "%20", " ")

	return path
}

// pathToURI converts a file system path to a file:// URI
func pathToURI(path string) string {
	// Ensure absolute path
	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err == nil {
			path = absPath
		}
	}

	// Simple encoding - a full URL encode would be better
	path = strings.ReplaceAll(path, " ", "%20")

	return "file://" + path
}
