# Goverhaul LSP Server

Real-time architecture violation detection for any LSP-compatible editor.

## Overview

The goverhaul LSP (Language Server Protocol) server provides real-time diagnostics for architecture violations directly in your editor. As you write code, violations are highlighted inline, just like syntax errors or linter warnings.

## Features

- **Real-time Diagnostics**: See violations as you code
- **Multi-Severity Support**: Error, Warning, Info, and Hint levels
- **Position-Accurate**: Violations highlighted at exact import locations
- **Configuration Hot-Reload**: Changes to `.goverhaul.yml` applied instantly
- **High Performance**: Uses MUS cache for sub-50ms response times
- **Editor Agnostic**: Works with VS Code, Vim, Emacs, and any LSP client

## Installation

### From Source

```bash
cd /home/alexrios/dev/goverhaul-lsp-server
go build -o goverhaul-lsp ./cmd/goverhaul-lsp
sudo mv goverhaul-lsp /usr/local/bin/
```

### Using Go Install

```bash
go install github.com/gophersatwork/goverhaul/cmd/goverhaul-lsp@latest
```

## Editor Setup

### VS Code

Create `.vscode/settings.json` in your workspace:

```json
{
  "goverhaul.enable": true,
  "goverhaul.configPath": ".goverhaul.yml",
  "goverhaul.analyzeOnSave": true,
  "goverhaul.analyzeOnChange": false
}
```

Install the extension (if available) or configure a custom LSP client:

```json
{
  "languageServerExample.trace.server": "verbose",
  "customLanguageServers": {
    "goverhaul": {
      "command": "goverhaul-lsp",
      "args": [],
      "filetypes": ["go"],
      "initializationOptions": {}
    }
  }
}
```

### Neovim (with nvim-lspconfig)

Add to your `init.lua`:

```lua
local lspconfig = require('lspconfig')
local configs = require('lspconfig.configs')

-- Define goverhaul LSP if not already defined
if not configs.goverhaul then
  configs.goverhaul = {
    default_config = {
      cmd = {'goverhaul-lsp'},
      filetypes = {'go'},
      root_dir = lspconfig.util.root_pattern('.goverhaul.yml', 'config.yml', 'go.mod'),
      settings = {},
    },
  }
end

-- Setup goverhaul LSP
lspconfig.goverhaul.setup{}
```

### Vim (with vim-lsp)

Add to your `.vimrc`:

```vim
if executable('goverhaul-lsp')
  au User lsp_setup call lsp#register_server({
    \ 'name': 'goverhaul',
    \ 'cmd': {server_info->['goverhaul-lsp']},
    \ 'allowlist': ['go'],
    \ 'workspace_config': {},
    \ })
endif
```

### Emacs (with lsp-mode)

Add to your Emacs configuration:

```elisp
(require 'lsp-mode)

(lsp-register-client
 (make-lsp-client
  :new-connection (lsp-stdio-connection "goverhaul-lsp")
  :major-modes '(go-mode)
  :server-id 'goverhaul
  :initialization-options (lambda ()
                            (list :configPath ".goverhaul.yml"))))

(add-hook 'go-mode-hook #'lsp)
```

### Sublime Text (with LSP package)

Add to `LSP.sublime-settings`:

```json
{
  "clients": {
    "goverhaul": {
      "enabled": true,
      "command": ["goverhaul-lsp"],
      "selector": "source.go",
      "settings": {
        "configPath": ".goverhaul.yml"
      }
    }
  }
}
```

## Configuration

The LSP server reads the same `.goverhaul.yml` configuration file as the CLI tool.

Example `.goverhaul.yml`:

```yaml
rules:
  - path: internal/api
    prohibited:
      - name: internal/database
        cause: API layer must not access database directly
        severity: error
      - name: fmt
        cause: Use structured logging instead
        severity: warning
    allowed:
      - internal/domain
      - internal/service

  - path: internal/domain
    prohibited:
      - name: internal/infrastructure
        cause: Domain should not depend on infrastructure
        severity: error

modfile: go.mod
incremental: true
cache_file: .goverhaul-cache.bin
```

### Severity Levels

- **error** (red): Blocks build, critical violation
- **warning** (yellow): Should be fixed, but not blocking
- **info** (blue): Informational, for awareness
- **hint** (gray): Suggestion for improvement

## How It Works

1. **Initialization**: LSP client sends `initialize` request with workspace root
2. **Configuration Loading**: Server reads `.goverhaul.yml` from workspace root
3. **File Events**: Client notifies server of file opens, changes, and saves
4. **Analysis**: Server runs goverhaul linter on Go files
5. **Diagnostics**: Violations converted to LSP diagnostics and published
6. **Hot Reload**: Server watches for config changes and re-analyzes

### Communication Protocol

The LSP server communicates via JSON-RPC 2.0 over stdio.

**Supported LSP Methods:**

- `initialize` - Start session, exchange capabilities
- `initialized` - Client ready notification
- `textDocument/didOpen` - File opened in editor
- `textDocument/didChange` - File content changed (incremental)
- `textDocument/didSave` - File saved to disk
- `textDocument/publishDiagnostics` - Send violations to editor
- `shutdown` - Clean shutdown
- `exit` - Terminate process

## Performance

The LSP server is optimized for real-time use:

- **<50ms** response time for diagnostic updates
- **MUS cache** for instant results on unchanged files
- **Incremental analysis** - only changed files re-analyzed
- **Efficient parsing** - uses Go's parser in `ImportsOnly` mode

### Benchmarks

```bash
cd lsp
go test -bench=. -benchmem

BenchmarkInitialize-8              50000    25000 ns/op    8192 B/op   45 allocs/op
BenchmarkViolationToDiagnostic-8  500000     2500 ns/op    1024 B/op   15 allocs/op
```

## Debugging

Enable verbose logging to stderr (LSP uses stdout for protocol):

```bash
# Run manually to see logs
goverhaul-lsp 2> /tmp/goverhaul-lsp.log

# In another terminal
tail -f /tmp/goverhaul-lsp.log
```

For editor-specific debugging:

- **VS Code**: Set `"goverhaul.trace.server": "verbose"` in settings
- **Neovim**: Use `:LspLog` to view LSP logs
- **Vim**: Set `let g:lsp_log_file = '/tmp/vim-lsp.log'`

## Troubleshooting

### No diagnostics appearing

1. Check that `.goverhaul.yml` exists in workspace root
2. Verify LSP server is running (check editor logs)
3. Ensure file is a Go file (`.go` extension)
4. Check server logs for configuration errors

### Diagnostics at wrong positions

1. Ensure using goverhaul with position support (latest version)
2. Check that file encoding is UTF-8
3. Verify line endings (LF, not CRLF)

### Slow performance

1. Enable incremental analysis in `.goverhaul.yml`
2. Check cache file is being created (`.goverhaul-cache.bin`)
3. Reduce number of rules if analyzing large codebase

### Configuration not reloading

1. Save the `.goverhaul.yml` file (LSP watches for save events)
2. Check editor is sending `didSave` notifications
3. Restart LSP server if hot-reload fails

## Examples

### Example Diagnostic Output

When you open a file with violations:

```json
{
  "uri": "file:///path/to/internal/api/handler.go",
  "diagnostics": [
    {
      "range": {
        "start": {"line": 4, "character": 1},
        "end": {"line": 4, "character": 25}
      },
      "severity": 1,
      "code": "api-no-db",
      "source": "goverhaul",
      "message": "import \"internal/database\" violates rule \"api-no-db\": API layer must not access database directly"
    }
  ]
}
```

This appears in your editor as:

```go
package api

import (
    "context"
    "internal/database"  // <- Red squiggly line with error message
)
```

### Multiple Violations

```go
package api

import (
    "fmt"                 // <- Yellow squiggly: Use structured logging instead
    "internal/database"   // <- Red squiggly: API layer must not access database
    "internal/cache"      // <- Blue info: Consider using domain types
)
```

## Future Enhancements

Planned features for future releases:

- **Code Actions**: Quick fixes to remove imports or update config
- **Hover Provider**: Show rule documentation on hover
- **Definition Provider**: Jump to rule definition in config
- **Code Lens**: Show violation counts at package level
- **Workspace Symbols**: Search for rules across workspace
- **Signature Help**: Suggest allowed imports while typing

## Architecture

```
┌─────────────────┐
│  LSP Client     │  (VS Code, Neovim, etc.)
│  (Editor)       │
└────────┬────────┘
         │ JSON-RPC 2.0 (stdio)
         │
┌────────▼────────┐
│  LSP Server     │  cmd/goverhaul-lsp/main.go
│  (this package) │
└────────┬────────┘
         │
┌────────▼────────┐
│  Goverhaul      │  Core linter engine
│  Linter         │  pkg/goverhaul/linter.go
└────────┬────────┘
         │
┌────────▼────────┐
│  MUS Cache      │  High-performance cache
│  (Granular)     │  cache.go
└─────────────────┘
```

## Contributing

Contributions welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

Areas for contribution:
- Code actions (quick fixes)
- Hover provider (rule documentation)
- VS Code extension
- Performance optimizations
- Additional editor integrations

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.

## Support

- Issues: https://github.com/gophersatwork/goverhaul/issues
- Discussions: https://github.com/gophersatwork/goverhaul/discussions
- Documentation: https://github.com/gophersatwork/goverhaul/wiki
