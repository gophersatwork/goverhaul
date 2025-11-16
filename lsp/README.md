# Goverhaul LSP Server

The Goverhaul Language Server Protocol (LSP) integration provides real-time linting and automated quick fixes for import rule violations directly in your IDE.

## Features

### Real-time Diagnostics
The LSP server analyzes your Go files as you edit them and displays violations as diagnostic messages (squiggly underlines) in your editor.

### Quick Fixes (Code Actions)

Goverhaul LSP provides three types of automated quick fixes for violations:

#### 1. Remove Import
Deletes the violating import statement from your file.

**Use when:**
- The import is no longer needed
- You can easily refactor the code to not use this dependency

**Example:**
```go
// Before
import (
    "fmt"
    "internal/database"  // ← Violation: squiggly underline
)

// After applying "Remove import" quick fix
import (
    "fmt"
)
```

**Note:** You'll need to manually refactor any code that uses the removed import.

#### 2. Add to Allowed List
Updates your `.goverhaul.yml` configuration file to allow this import for the current path.

**Use when:**
- The violation is a false positive
- You've verified this import is acceptable for this specific use case
- The import should be permanently allowed in this package

**Example:**
Before applying the fix, your config might look like:
```yaml
rules:
  - path: "internal/api"
    prohibited:
      - name: "internal/database"
        cause: "Use repository pattern"
```

After applying "Add to allowed list" quick fix:
```yaml
rules:
  - path: "internal/api"
    allowed:
      - "internal/database"  # ← Added automatically
    prohibited:
      - name: "internal/database"
        cause: "Use repository pattern"
```

**Note:** This modifies your configuration file. The import will no longer be flagged in this path.

#### 3. Suppress Violation
Adds a `// goverhaul:ignore` comment above the import to suppress the specific violation.

**Use when:**
- Dealing with legacy code that will be refactored later
- Temporarily need to bypass the rule
- The violation is acknowledged but can't be fixed immediately

**Example:**
```go
// Before
import "internal/database"

// After applying "Suppress this violation" quick fix
// goverhaul:ignore - Legacy code, will refactor
import "internal/database"
```

**Note:** Use this sparingly. It's better to fix the underlying issue or update the configuration.

## Usage in VS Code

1. Open a Go file with import violations
2. Position your cursor on the violation (indicated by a squiggly underline)
3. Press `Ctrl+.` (Windows/Linux) or `Cmd+.` (macOS) to open the quick fix menu
4. Select the desired fix from the list
5. The file (and potentially config file) will be updated automatically

## Usage in Other Editors

The LSP server follows the standard LSP protocol, so it works with any LSP-compatible editor:

- **Neovim/Vim**: Use `nvim-lspconfig` or similar
- **Emacs**: Use `lsp-mode` or `eglot`
- **Sublime Text**: Use LSP package
- **IntelliJ/GoLand**: Use the LSP plugin

## Implementation Details

### Code Action Provider
The `CodeActionProvider` is responsible for generating quick fixes based on diagnostics:

```go
provider := NewCodeActionProvider(fs, configPath)
actions := provider.GetCodeActions(uri, diagnostic)
```

### Import Parser
The import parser extracts import paths from diagnostic messages and locates them in source files:

```go
importPath := ExtractImportPath(message)
importRange, isSingle, err := GetImportBlockRange(fs, filePath, importPath)
```

### Config Editor
The config editor programmatically modifies YAML configuration files while preserving structure:

```go
editor := NewConfigEditor(fs, configPath)
edits, err := editor.AddToAllowedList(rulePath, importPath)
```

## Architecture

The LSP server is built on top of the existing Goverhaul linter:

```
┌─────────────────┐
│   IDE Client    │
└────────┬────────┘
         │ LSP Protocol (JSON-RPC)
         │
┌────────▼────────┐
│   LSP Server    │
│                 │
│  ┌───────────┐  │
│  │ Diagnostics│  │
│  │  Generator │  │
│  └─────┬─────┘  │
│        │        │
│  ┌─────▼─────┐  │
│  │Code Action │  │
│  │ Provider  │  │
│  └─────┬─────┘  │
│        │        │
└────────┼────────┘
         │
┌────────▼────────┐
│ Goverhaul Linter│
└─────────────────┘
```

## Testing

Run the LSP tests:

```bash
go test ./lsp/... -v
```

The test suite covers:
- Code action generation
- Import path extraction
- Config file editing
- Diagnostic creation
- LSP server handlers

## Future Enhancements

Potential future features:
- **Smart refactoring**: Automatically refactor code when removing imports
- **Batch fixes**: Apply fixes to multiple files at once
- **Custom fix suggestions**: Suggest alternative imports
- **Rule suggestions**: Recommend rule configurations based on violations
- **Interactive config editing**: Visual UI for managing rules

## Contributing

When adding new code actions:

1. Implement the action in `code_actions.go`
2. Add corresponding tests in `code_actions_test.go`
3. Update this documentation
4. Ensure all tests pass

## License

Same as the main Goverhaul project.
