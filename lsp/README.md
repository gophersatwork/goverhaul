# Goverhaul LSP Server

A Language Server Protocol (LSP) implementation for Goverhaul that provides real-time architecture rule feedback in your editor.

## Features

### Hover Documentation

Hover over any import statement to see:
- Whether the import violates architecture rules
- Rule details including severity and cause
- Whether the import is explicitly allowed
- Rule location in configuration

## Usage

### As a Library

```go
package main

import (
    "context"
    "log/slog"
    "os"

    "github.com/gophersatwork/goverhaul"
    "github.com/gophersatwork/goverhaul/lsp"
    "github.com/spf13/afero"
)

func main() {
    // Load configuration
    config, err := goverhaul.LoadConfig(afero.NewOsFs(), ".", ".goverhaul.yml")
    if err != nil {
        log.Fatal(err)
    }

    // Create linter
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
    linter, err := goverhaul.NewLinter(config, logger, afero.NewOsFs())
    if err != nil {
        log.Fatal(err)
    }

    // Start LSP server
    server := lsp.NewServer(linter, &config, afero.NewOsFs(), logger, os.Stdin, os.Stdout)
    if err := server.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

## LSP Capabilities

### textDocument/hover

Provides hover information for import statements.

**Request:**
```json
{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "textDocument/hover",
    "params": {
        "textDocument": {
            "uri": "file:///path/to/file.go"
        },
        "position": {
            "line": 3,
            "character": 5
        }
    }
}
```

**Response for violated import:**
```json
{
    "jsonrpc": "2.0",
    "id": 1,
    "result": {
        "contents": {
            "kind": "markdown",
            "value": "# Import: `internal/database`\n\n## Violations\n\n### ❌ Rule: internal/api\n\n**Cause**: API layer must not access database directly\n\n---\n\n**Rule Path**: `internal/api`\n\n*Defined in `.goverhaul.yml`*\n"
        },
        "range": {
            "start": { "line": 3, "character": 1 },
            "end": { "line": 3, "character": 20 }
        }
    }
}
```

**Response for allowed import:**
```json
{
    "jsonrpc": "2.0",
    "id": 1,
    "result": {
        "contents": {
            "kind": "markdown",
            "value": "# Import: `context`\n\n## Allowed\n\nThis import is allowed for files in `internal/api`\n\n---\n\n**Rule Path**: `internal/api`\n\n*Defined in `.goverhaul.yml`*\n"
        },
        "range": {
            "start": { "line": 3, "character": 1 },
            "end": { "line": 3, "character": 10 }
        }
    }
}
```

**Response for unconfigured import:**
```json
{
    "jsonrpc": "2.0",
    "id": 1,
    "result": {
        "contents": {
            "kind": "markdown",
            "value": "# Import: `encoding/json`\n\n## No Rules\n\nNo architecture rules defined for this import.\n"
        },
        "range": {
            "start": { "line": 3, "character": 1 },
            "end": { "line": 3, "character": 16 }
        }
    }
}
```

## Hover Content Examples

### Violation Example

When you hover over a prohibited import:

```markdown
# Import: `internal/database`

## Violations

### ❌ Rule: internal/api

**Cause**: API layer must not access database directly

Use a service layer or repository pattern to interact with the database.
This maintains separation of concerns and makes testing easier.

---

**Rule Path**: `internal/api`

*Defined in `.goverhaul.yml`*
```

### Allowed Import Example

When you hover over an explicitly allowed import:

```markdown
# Import: `context`

## Allowed

This import is allowed for files in `internal/api`

---

**Rule Path**: `internal/api`

*Defined in `.goverhaul.yml`*
```

### No Rules Example

When you hover over an import with no rules:

```markdown
# Import: `encoding/json`

## No Rules

No architecture rules defined for this import.
```

## Editor Integration

### VS Code

1. Install the Go extension
2. Configure your LSP server to use goverhaul
3. Hover over imports to see rule violations

### Neovim

Configure with nvim-lspconfig:

```lua
require('lspconfig').goverhaul.setup{
    cmd = {'goverhaul-lsp'},
    filetypes = {'go'},
}
```

## Architecture

The hover provider works by:

1. Parsing the Go file at the cursor position
2. Identifying if the cursor is on an import statement
3. Running the linter to check for violations
4. Looking up rule information from the config
5. Building markdown-formatted hover content
6. Returning the hover information to the editor

## Performance

- Hover responses are generated in <10ms on average
- File parsing uses go/parser with `ImportsOnly` mode for speed
- Linting results are cached when incremental mode is enabled
- Path normalization ensures consistent rule matching

## Testing

Run the test suite:

```bash
go test ./lsp/... -v
```

Run with coverage:

```bash
go test ./lsp/... -cover
```

## Contributing

Contributions are welcome! Please ensure:
- All tests pass
- New features include tests
- Code follows Go best practices
- Documentation is updated
