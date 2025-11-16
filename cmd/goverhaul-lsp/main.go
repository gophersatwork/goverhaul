package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/gophersatwork/goverhaul/lsp"
	"github.com/sourcegraph/jsonrpc2"
)

func main() {
	// Set up logging to stderr (stdout is reserved for JSON-RPC)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("Starting goverhaul LSP server")

	ctx := context.Background()

	// Create server instance
	server := lsp.NewServer(ctx, logger)

	// Create JSON-RPC connection using stdio
	stream := jsonrpc2.NewBufferedStream(stdrwc{}, jsonrpc2.VSCodeObjectCodec{})
	conn := jsonrpc2.NewConn(ctx, stream, jsonrpc2.HandlerWithError(server.Handle))

	logger.Info("LSP server listening on stdio")

	// Wait for connection to close
	<-conn.DisconnectNotify()

	logger.Info("LSP server disconnected")
}

// stdrwc implements io.ReadWriteCloser for stdin/stdout
type stdrwc struct{}

func (stdrwc) Read(p []byte) (int, error) {
	return os.Stdin.Read(p)
}

func (stdrwc) Write(p []byte) (int, error) {
	n, err := os.Stdout.Write(p)
	if err != nil {
		// Log to stderr
		fmt.Fprintf(os.Stderr, "Error writing to stdout: %v\n", err)
	}
	return n, err
}

func (stdrwc) Close() error {
	if err := os.Stdin.Close(); err != nil {
		return err
	}
	return os.Stdout.Close()
}
