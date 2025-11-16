package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/gophersatwork/goverhaul"
	"github.com/spf13/afero"
)

// Server represents the LSP server
type Server struct {
	linter     *goverhaul.Goverhaul
	config     *goverhaul.Config
	configPath string
	logger     *slog.Logger
	fs         afero.Fs

	reader io.Reader
	writer io.Writer
}

// NewServer creates a new LSP server
func NewServer(
	linter *goverhaul.Goverhaul,
	config *goverhaul.Config,
	configPath string,
	logger *slog.Logger,
	fs afero.Fs,
	reader io.Reader,
	writer io.Writer,
) *Server {
	return &Server{
		linter:     linter,
		config:     config,
		configPath: configPath,
		logger:     logger,
		fs:         fs,
		reader:     reader,
		writer:     writer,
	}
}

// Start starts the LSP server
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting Goverhaul LSP server")

	// In a real implementation, we would:
	// 1. Read JSON-RPC messages from stdin
	// 2. Parse them and dispatch to handlers
	// 3. Send responses back via stdout
	//
	// For this implementation, we'll provide the handler methods
	// that would be called by the JSON-RPC dispatcher

	return nil
}

// TextDocumentDidOpen handles document open notifications
func (s *Server) TextDocumentDidOpen(params struct {
	TextDocument struct {
		URI  string `json:"uri"`
		Text string `json:"text"`
	} `json:"textDocument"`
}) error {
	s.logger.Debug("Document opened", "uri", params.TextDocument.URI)

	// Lint the document and publish diagnostics
	return s.lintAndPublishDiagnostics(params.TextDocument.URI)
}

// TextDocumentDidChange handles document change notifications
func (s *Server) TextDocumentDidChange(params struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}) error {
	s.logger.Debug("Document changed", "uri", params.TextDocument.URI)

	// Lint the document and publish diagnostics
	return s.lintAndPublishDiagnostics(params.TextDocument.URI)
}

// TextDocumentDidSave handles document save notifications
func (s *Server) TextDocumentDidSave(params struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}) error {
	s.logger.Debug("Document saved", "uri", params.TextDocument.URI)

	// Lint the document and publish diagnostics
	return s.lintAndPublishDiagnostics(params.TextDocument.URI)
}

// TextDocumentCodeAction handles code action requests
func (s *Server) TextDocumentCodeAction(params CodeActionParams) ([]CodeAction, error) {
	s.logger.Debug("Code action requested",
		"uri", params.TextDocument.URI,
		"range", params.Range,
		"diagnostics_count", len(params.Context.Diagnostics))

	provider := NewCodeActionProvider(s.fs, s.configPath)

	var actions []CodeAction
	for _, diag := range params.Context.Diagnostics {
		if diag.Source == "goverhaul" {
			codeActions := provider.GetCodeActions(
				params.TextDocument.URI,
				diag,
			)
			actions = append(actions, codeActions...)
		}
	}

	s.logger.Debug("Code actions generated", "count", len(actions))

	return actions, nil
}

// lintAndPublishDiagnostics lints a document and publishes diagnostics
func (s *Server) lintAndPublishDiagnostics(uri string) error {
	// Convert URI to file path
	filePath := strings.TrimPrefix(uri, "file://")

	s.logger.Debug("Linting file", "path", filePath)

	// Lint the file
	violations, err := s.linter.Lint(filePath)
	if err != nil {
		s.logger.Error("Failed to lint file", "path", filePath, "error", err)
		return err
	}

	// Convert violations to diagnostics
	diagnostics := s.violationsToDiagnostics(violations, filePath)

	// Publish diagnostics
	params := PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	}

	return s.publishDiagnostics(params)
}

// violationsToDiagnostics converts lint violations to LSP diagnostics
func (s *Server) violationsToDiagnostics(violations *goverhaul.LintViolations, filePath string) []Diagnostic {
	if violations == nil || violations.IsEmpty() {
		return []Diagnostic{}
	}

	diagnostics := make([]Diagnostic, 0)

	for _, v := range violations.Violations {
		// Only include violations for this file
		if v.File != filePath {
			continue
		}

		// Try to get the exact range of the import
		importRange, _, err := GetImportBlockRange(s.fs, filePath, v.Import)
		if err != nil {
			// Fallback to line 0 if we can't find the import
			importRange = Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 0, Character: 0},
			}
		}

		message := fmt.Sprintf("import \"%s\" violates rule for path \"%s\"", v.Import, v.Rule)
		if v.Cause != "" {
			message += fmt.Sprintf(": %s", v.Cause)
		}
		if v.Details != "" {
			message += fmt.Sprintf(" - %s", v.Details)
		}

		diagnostic := Diagnostic{
			Range:    importRange,
			Severity: DiagnosticSeverityError,
			Source:   "goverhaul",
			Message:  message,
			Code:     "import-violation",
		}

		diagnostics = append(diagnostics, diagnostic)
	}

	return diagnostics
}

// publishDiagnostics sends diagnostics to the client
func (s *Server) publishDiagnostics(params PublishDiagnosticsParams) error {
	s.logger.Debug("Publishing diagnostics",
		"uri", params.URI,
		"count", len(params.Diagnostics))

	// In a real implementation, this would send a JSON-RPC notification
	// to the client via stdout
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "textDocument/publishDiagnostics",
		"params":  params,
	}

	data, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Write to output (in real implementation, this would be properly formatted
	// with Content-Length header, etc.)
	s.logger.Debug("Diagnostics notification", "data", string(data))

	return nil
}
