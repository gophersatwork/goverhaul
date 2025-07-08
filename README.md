# Goverhaul

Goverhaul is a CLI tool to enforce architectural rules in Go projects. It helps teams maintain the intended architecture by defining and enforcing import boundaries between packages.

## Features

- Define allowed imports for specific package paths
- Define prohibited imports for specific package paths with explanatory causes
- Generate visual dependency graphs to better understand architectural violations
- Simple YAML configuration
- Easy integration into CI/CD pipelines

## Installation

### Using Go Install

```bash
go install github.com/alexrios/goverhaul@latest
```

### From Source

```bash
git clone https://github.com/alexrios/goverhaul.git && cd goverhaul && go build
```

## Usage

Run goverhaul in your project directory:

```bash
goverhaul --path . --config .goverhaul.yml
```

### Quick Start Guide

1. Install Goverhaul:
   ```bash
   go install github.com/alexrios/goverhaul@latest
   ```

2. Create a `.goverhaul.yml` file in your project root:
   ```yaml
   # Basic configuration for a typical layered architecture
   rules:
     # Domain layer should not depend on infrastructure
     - path: "internal/domain"
       prohibited:
         - name: "internal/infrastructure"
           cause: "Domain should not depend on infrastructure"

     # API layer should not access database directly
     - path: "internal/api"
       prohibited:
         - name: "internal/database"
           cause: "API should access database through domain services"
   ```

3. Run Goverhaul:
   ```bash
   goverhaul --path . --config .goverhaul.yml
   ```

4. Integrate with CI/CD (example GitHub Actions workflow):
   ```yaml
   # .github/workflows/architecture.yml
   name: Architecture Check

   on:
     push:
       branches: [ main ]
     pull_request:
       branches: [ main ]

   jobs:
     goverhaul:
       runs-on: ubuntu-latest
       steps:
       - uses: actions/checkout@v3

       - name: Set up Go
         uses: actions/setup-go@v4
         with:
           go-version: '1.24'

       - name: Install Goverhaul
         run: go install github.com/alexrios/goverhaul@latest

       - name: Check architecture
         run: goverhaul --path . --config .goverhaul.yml
   ```

### Command Line Options

- `--path`: Path to lint (default: ".")
- `--config`: Path to config file (default: "$HOME/.goverhaul.yml")
- `--verbose`: Enable verbose logging for debugging

## Configuration

Goverhaul uses a YAML configuration file to define architectural rules. Create a `.goverhaul.yml` file in your project or home directory.

### Example Configuration

```yaml
modfile: "go.mod"  # Optional: Path to go.mod file (default: "go.mod")
incremental: true  # Optional: Enable incremental analysis (default: false)
cache_file: ".goverhaul.cache"  # Optional: Path to cache file (default: "$HOME/.goverhaul.cache")
rules:
  - path: "pkg/api"
    allowed:
      - "fmt"
      - "net/http"
  - path: "pkg/db"
    prohibited:
      - name: "pkg/api"
        cause: "Database layer should not depend on API layer"
```

### Configuration Options

- `modfile`: Optional path to the go.mod file (default: `go.mod`)
- `incremental`: Optional boolean to enable incremental analysis for faster subsequent runs (default: `true`)
- `cache_file`: Optional path to the cache file for incremental analysis (default: `$HOME/.goverhaul/cache.json`)
- `rules`: List of architectural rules to enforce
  - `path`: Package path to apply the rule to
  - `allowed`: List of allowed imports
  - `prohibited`: List of prohibited imports
    - `name`: Package name to prohibit
    - `cause`: Explanation for why the import is prohibited

### How Rules Work

- If `allowed` is specified, only those imports are permitted for the package
- If `prohibited` is specified, those imports are not allowed for the package
- Rules are applied to all Go files in the specified path and its subdirectories
- Import paths can be standard library packages, third-party packages, or internal packages
- For internal packages, you can use either the full import path (including module name) or the relative path

### Advanced Rule Examples

#### Enforcing Architecture

```yaml
rules:
  # Domain layer can only import standard library
  - path: "internal/domain"
    allowed:
      - "fmt"
      - "errors"
      - "context"
      - "time"
      - "encoding/json"

  # Use case layer can import domain but not infrastructure
  - path: "internal/usecase"
    allowed:
      - "fmt"
      - "errors"
      - "context"
      - "time"
      - "internal/domain"
    prohibited:
      - name: "internal/infrastructure"
        cause: "Use cases should not depend directly on infrastructure and should declare their own interfaces"

  # Infrastructure layer can import domain but not use cases
  - path: "internal/infrastructure"
    prohibited:
      - name: "internal/usecase"
        cause: "Infrastructure should not depend on use cases"
```

#### Enforcing Module Boundaries

```yaml
rules:
  # Core module has no external dependencies
  - path: "pkg/core"
    prohibited:
      - name: "pkg/api"
        cause: "Core should not depend on API"
      - name: "pkg/db"
        cause: "Core should not depend on DB"
      - name: "pkg/auth"
        cause: "Core should not depend on Auth"

  # API module can use core but not DB directly
  - path: "pkg/api"
    prohibited:
      - name: "pkg/db"
        cause: "API should access DB through core interfaces"
```

## Best Practices for Defining Architectural Rules

### 1. Start with Clear Architectural Boundaries

Before defining rules, establish a clear architectural vision:
- Identify the main components/layers of your application
- Define the intended dependencies between components
- Document the architectural decisions and constraints

### 2. Be Explicit About Allowed Imports

For critical packages (like domain models or core business logic):
- Use the `allowed` list to explicitly whitelist permitted imports
- Include only necessary standard library packages
- Be conservative with third-party dependencies

### 3. Use Prohibited Imports for Boundary Enforcement

For packages with specific constraints:
- Use `prohibited` to prevent unwanted dependencies
- Always include a clear `cause` explaining the architectural constraint
- Focus on preventing dependency cycles and maintaining layer separation

### 4. Organize Rules by Architectural Concerns

Group rules logically:
- Layer-based rules (presentation, domain, data)
- Feature module boundaries
- Cross-cutting concerns (security, logging, etc.)

### 5. Evolve Rules Incrementally

As your project grows:
- Start with a minimal set of critical rules
- Add new rules as architectural patterns emerge
- Refine existing rules based on team feedback
- Use the incremental analysis feature for faster feedback in large codebases

### 6. Document Architectural Intent

Use the `cause` field effectively:
- Explain the architectural principle being enforced
- Reference design patterns or architectural styles
- Link to team documentation or discussions

## Use Cases

### Enforcing Clean/Hexagonal Architecture

Goverhaul helps maintain the dependency rule in clean architecture:
- Domain entities have no external dependencies
- Use cases depend only on domain entities
- Interface adapters depend on use cases but not frameworks
- Frameworks and drivers are isolated at the boundaries

### Preventing Dependency Cycles

Detect and prevent circular dependencies between packages:
- Identify potential cycles in your dependency graph
- Define rules to enforce a directed acyclic graph (DAG)
- Ensure changes don't introduce new cycles

### Maintaining Module Boundaries

For multi-module projects:
- Define clear boundaries between modules
- Enforce API contracts between modules
- Prevent implementation details from leaking across module boundaries

### Controlling Third-Party Dependencies

Limit the spread of external dependencies:
- Restrict which packages can import specific third-party libraries
- Isolate framework dependencies to adapter layers
- Prevent core business logic from depending on external packages

### Documenting Architectural Decisions

Use rules as executable documentation:
- Encode architectural decisions as enforceable rules
- Make architectural constraints visible to the team
- Ensure new team members understand the intended architecture


## Troubleshooting

### Common Issues and Solutions

#### Rule Not Being Applied

**Issue**: You've defined a rule, but it doesn't seem to be applied to your code.

**Solutions**:
- Verify that the `path` in your rule matches your project's package structure
- Check that you're running Goverhaul with the correct `--path` argument
- Use the `--verbose` flag to see which files are being analyzed
- Ensure your Go files have proper package declarations

#### Multiple Rule Matches

**Issue**: You're getting unexpected results because multiple rules are matching the same package.

**Solutions**:
- Make your rule paths more specific
- Review your rule order (**rules are evaluated in the order they appear in the config**)
- Use the `--verbose` flag to see which rules are being applied

#### Incremental Analysis Issues

**Issue**: Incremental analysis is not detecting changes or is skipping files that should be analyzed.

**Solutions**:
- Delete the cache file and run again
- Specify a custom cache file location with the `cache_file` option
- Disable incremental analysis if you're experiencing issues

#### Integration with CI/CD

**Issue**: Goverhaul is failing in CI but works locally.

**Solutions**:
- Ensure your CI environment has the correct Go version
- Check that your configuration file is being properly included in your repository
- Use absolute paths in your CI configuration
- Add debug output with the `--verbose` flag


## License

[MIT License](LICENSE)

## Contributing

Contributions are welcome! Please feel free to open a discussion.