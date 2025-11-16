package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/gophersatwork/goverhaul"
	"github.com/spf13/afero"
)

// Server represents an LSP server for goverhaul
type Server struct {
	linter *goverhaul.Goverhaul
	config *goverhaul.Config
	fs     afero.Fs
	logger *slog.Logger
	reader io.Reader
	writer io.Writer
}

// NewServer creates a new LSP server
func NewServer(
	linter *goverhaul.Goverhaul,
	config *goverhaul.Config,
	fs afero.Fs,
	logger *slog.Logger,
	reader io.Reader,
	writer io.Writer,
) *Server {
	return &Server{
		linter: linter,
		config: config,
		fs:     fs,
		logger: logger,
		reader: reader,
		writer: writer,
	}
}

// TextDocumentHover handles hover requests
func (s *Server) TextDocumentHover(ctx context.Context, params TextDocumentPositionParams) (*Hover, error) {
	s.logger.Debug("Hover request received",
		"uri", params.TextDocument.URI,
		"line", params.Position.Line,
		"character", params.Position.Character)

	provider := NewHoverProvider(s.linter, s.config, s.fs)

	hover, err := provider.GetHover(
		params.TextDocument.URI,
		params.Position,
	)

	if err != nil {
		s.logger.Error("Hover request failed", "error", err)
		return nil, err
	}

	if hover != nil {
		s.logger.Debug("Hover content generated",
			"uri", params.TextDocument.URI,
			"contentLength", len(hover.Contents.Value))
	} else {
		s.logger.Debug("No hover content (not on import)",
			"uri", params.TextDocument.URI)
	}

	return hover, nil
}

// Request represents a JSON-RPC request
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC response
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

// Error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// Start starts the LSP server
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting LSP server")

	decoder := json.NewDecoder(s.reader)
	encoder := json.NewEncoder(s.writer)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("LSP server shutting down")
			return ctx.Err()
		default:
		}

		var req Request
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				s.logger.Info("Client disconnected")
				return nil
			}
			s.logger.Error("Failed to decode request", "error", err)
			continue
		}

		s.logger.Debug("Request received", "method", req.Method, "id", req.ID)

		resp := s.handleRequest(ctx, req)
		if resp != nil {
			if err := encoder.Encode(resp); err != nil {
				s.logger.Error("Failed to encode response", "error", err)
				continue
			}
		}
	}
}

// handleRequest handles a single LSP request
func (s *Server) handleRequest(ctx context.Context, req Request) *Response {
	switch req.Method {
	case "textDocument/hover":
		return s.handleHover(ctx, req)
	case "initialize":
		return s.handleInitialize(ctx, req)
	case "initialized":
		// Notification - no response needed
		return nil
	case "shutdown":
		return s.handleShutdown(ctx, req)
	case "exit":
		// Notification - no response needed
		return nil
	default:
		s.logger.Warn("Method not found", "method", req.Method)
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    MethodNotFound,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

// handleHover handles textDocument/hover requests
func (s *Server) handleHover(ctx context.Context, req Request) *Response {
	var params TextDocumentPositionParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    InvalidParams,
				Message: "Invalid hover parameters",
				Data:    err.Error(),
			},
		}
	}

	hover, err := s.TextDocumentHover(ctx, params)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    InternalError,
				Message: "Hover request failed",
				Data:    err.Error(),
			},
		}
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  hover,
	}
}

// handleInitialize handles initialize requests
func (s *Server) handleInitialize(ctx context.Context, req Request) *Response {
	s.logger.Info("Initialize request received")

	result := map[string]interface{}{
		"capabilities": map[string]interface{}{
			"hoverProvider": true,
			"textDocumentSync": map[string]interface{}{
				"openClose": true,
				"change":    1, // Full sync
			},
		},
		"serverInfo": map[string]string{
			"name":    "goverhaul-lsp",
			"version": "0.1.0",
		},
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// handleShutdown handles shutdown requests
func (s *Server) handleShutdown(ctx context.Context, req Request) *Response {
	s.logger.Info("Shutdown request received")

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  nil,
	}
}
