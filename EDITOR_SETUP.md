# Goverhaul LSP Editor Setup Guide

Quick setup guide for popular editors. For full documentation, see [LSP.md](LSP.md).

## Prerequisites

```bash
# Build and install LSP server
cd /home/alexrios/dev/goverhaul-lsp-server
go build -o goverhaul-lsp ./cmd/goverhaul-lsp
sudo mv goverhaul-lsp /usr/local/bin/
```

Verify installation:
```bash
which goverhaul-lsp
```

## VS Code

1. Install a generic LSP client extension (e.g., "vscode-languageserver-node")
2. Add to workspace settings (.vscode/settings.json):

```json
{
  "go.toolsManagement.checkForUpdates": "proxy",
  "goverhaul.enable": true
}
```

## Neovim (nvim-lspconfig)

Add to your `init.lua`:

```lua
local lspconfig = require('lspconfig')
local configs = require('lspconfig.configs')

if not configs.goverhaul then
  configs.goverhaul = {
    default_config = {
      cmd = {'goverhaul-lsp'},
      filetypes = {'go'},
      root_dir = lspconfig.util.root_pattern('.goverhaul.yml', 'go.mod'),
      settings = {},
    },
  }
end

lspconfig.goverhaul.setup{}
```

## Vim (vim-lsp)

Add to `.vimrc`:

```vim
if executable('goverhaul-lsp')
  au User lsp_setup call lsp#register_server({
    \ 'name': 'goverhaul',
    \ 'cmd': {server_info->['goverhaul-lsp']},
    \ 'allowlist': ['go'],
    \ })
endif
```

## Test Installation

1. Create a test project with `.goverhaul.yml`:

```yaml
rules:
  - path: .
    prohibited:
      - name: fmt
        cause: Use structured logging instead
        severity: warning
modfile: go.mod
```

2. Create `test.go`:

```go
package main

import "fmt" // Should show warning

func main() {
    fmt.Println("test")
}
```

3. Open in your editor - you should see a warning on the fmt import!

## Troubleshooting

See [LSP.md](LSP.md#troubleshooting) for detailed troubleshooting steps.
