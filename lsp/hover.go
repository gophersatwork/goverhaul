package lsp

import (
	"fmt"
	"go/parser"
	"go/token"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gophersatwork/goverhaul"
	"github.com/spf13/afero"
)

// HoverProvider generates hover tooltips for imports
type HoverProvider struct {
	linter *goverhaul.Goverhaul
	config *goverhaul.Config
	fs     afero.Fs
}

// NewHoverProvider creates a new hover provider
func NewHoverProvider(linter *goverhaul.Goverhaul, config *goverhaul.Config, fs afero.Fs) *HoverProvider {
	return &HoverProvider{
		linter: linter,
		config: config,
		fs:     fs,
	}
}

// RuleInfo contains details about a rule from config
type RuleInfo struct {
	Path      string // The path pattern this rule applies to
	IsAllowed bool   // Whether this import is explicitly allowed
	Reason    string // Reason for prohibition or additional context
}

// GetHover returns documentation for the symbol at the given position
func (p *HoverProvider) GetHover(uri string, position Position) (*Hover, error) {
	// 1. Determine if position is on an import
	importPath, importRange := p.findImportAtPosition(uri, position)
	if importPath == "" {
		return nil, nil // Not on an import
	}

	// 2. Get file path from URI
	filePath, err := uriToPath(uri)
	if err != nil {
		return nil, err
	}

	// 3. Get violations for this file
	violations := p.getViolationsForImport(filePath, importPath)

	// 4. Get rule details from config
	ruleInfo := p.getRuleInfo(filePath, importPath)

	// 5. Build hover content
	content := p.buildHoverContent(importPath, violations, ruleInfo)

	return &Hover{
		Contents: content,
		Range:    &importRange,
	}, nil
}

// findImportAtPosition uses go/parser to find import at cursor
func (p *HoverProvider) findImportAtPosition(uri string, position Position) (string, Range) {
	filePath, err := uriToPath(uri)
	if err != nil {
		return "", Range{}
	}

	// Normalize and remove leading slash if present
	normalizedPath := goverhaul.NormalizePath(filePath)
	if strings.HasPrefix(normalizedPath, "/") {
		normalizedPath = strings.TrimPrefix(normalizedPath, "/")
	}

	// Read the file content
	content, err := afero.ReadFile(p.fs, normalizedPath)
	if err != nil {
		return "", Range{}
	}

	// Parse file
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, normalizedPath, content, parser.ImportsOnly)
	if err != nil {
		return "", Range{}
	}

	// Convert LSP position to token.Pos (LSP is 0-indexed, go/token is 1-indexed)
	targetLine := position.Line + 1

	// Find import spec at this line
	for _, importSpec := range file.Imports {
		pos := fset.Position(importSpec.Path.Pos())
		end := fset.Position(importSpec.Path.End())

		// Check if the position is on this import line
		if pos.Line == targetLine {
			importPath := strings.Trim(importSpec.Path.Value, `"`)
			return importPath, Range{
				Start: Position{Line: pos.Line - 1, Character: pos.Column - 1},
				End:   Position{Line: end.Line - 1, Character: end.Column - 1},
			}
		}
	}

	return "", Range{}
}

// getViolationsForImport gets violations for a specific import in a file
func (p *HoverProvider) getViolationsForImport(filePath, importPath string) []goverhaul.LintViolation {
	// Lint from current directory (project root) to get all violations
	// This ensures we pick up the go.mod and all rules correctly
	violations, err := p.linter.Lint(".")
	if err != nil {
		return nil
	}

	// Normalize file path for comparison (remove leading slash if present)
	normalizedPath := goverhaul.NormalizePath(filePath)
	if strings.HasPrefix(normalizedPath, "/") {
		normalizedPath = strings.TrimPrefix(normalizedPath, "/")
	}

	// Filter violations for this specific import and file
	var result []goverhaul.LintViolation
	for _, v := range violations.Violations {
		normalizedViolationFile := goverhaul.NormalizePath(v.File)
		if v.Import == importPath && normalizedViolationFile == normalizedPath {
			result = append(result, v)
		}
	}

	return result
}

// getRuleInfo finds the rule that applies to this import
func (p *HoverProvider) getRuleInfo(filePath, importPath string) *RuleInfo {
	// Normalize the file path and remove leading slash if present
	normalizedPath := goverhaul.NormalizePath(filePath)
	if strings.HasPrefix(normalizedPath, "/") {
		normalizedPath = strings.TrimPrefix(normalizedPath, "/")
	}

	// Find matching rule in config
	for _, rule := range p.config.Rules {
		if !ruleAppliesToPath(rule, normalizedPath) {
			continue
		}

		// Check if import is in allowed list
		for _, allowed := range rule.Allowed {
			if matchesImport(allowed, importPath) {
				return &RuleInfo{
					Path:      rule.Path,
					IsAllowed: true,
				}
			}
		}

		// Check if import is prohibited
		for _, prohibited := range rule.Prohibited {
			if matchesImport(prohibited.Name, importPath) {
				return &RuleInfo{
					Path:      rule.Path,
					IsAllowed: false,
					Reason:    prohibited.Cause,
				}
			}
		}
	}

	return nil
}

// ruleAppliesToPath checks if a rule applies to a given file path
func ruleAppliesToPath(rule goverhaul.Rule, filePath string) bool {
	rulePath := goverhaul.NormalizePath(rule.Path)
	currentDir := goverhaul.DirPath(filePath)

	// Check for exact match first
	if currentDir == rulePath {
		return true
	}

	// Check if it's a subdirectory
	if goverhaul.IsSubPath(rulePath, currentDir) {
		return true
	}

	// Handle relative paths - check if the absolute path matches
	if !goverhaul.IsAbsPath(rulePath) {
		absPath := goverhaul.AbsPath(filePath)
		absDir := goverhaul.DirPath(absPath)

		// Check if the absolute directory ends with the rule path
		if strings.HasSuffix(absDir, "/"+rulePath) || strings.HasSuffix(absDir, rulePath) {
			return true
		}

		// Also check subdirectory relationship with absolute path
		if goverhaul.IsSubPath(rulePath, absDir) {
			return true
		}
	}

	return false
}

// matchesImport checks if an import pattern matches an import path
func matchesImport(pattern, importPath string) bool {
	// Direct match
	if pattern == importPath {
		return true
	}

	// Wildcard match (simple glob pattern)
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(importPath, prefix+"/")
	}

	if strings.HasSuffix(pattern, "/...") {
		prefix := strings.TrimSuffix(pattern, "/...")
		return strings.HasPrefix(importPath, prefix+"/") || importPath == prefix
	}

	return false
}

// buildHoverContent creates markdown content for hover tooltip
func (p *HoverProvider) buildHoverContent(
	importPath string,
	violations []goverhaul.LintViolation,
	ruleInfo *RuleInfo,
) MarkupContent {
	var md strings.Builder

	md.WriteString(fmt.Sprintf("# Import: `%s`\n\n", importPath))

	if len(violations) > 0 {
		md.WriteString("## Violations\n\n")
		for _, v := range violations {
			icon := getSeverityIcon("error")
			md.WriteString(fmt.Sprintf("### %s Rule: %s\n\n", icon, v.Rule))

			if v.Cause != "" {
				md.WriteString(fmt.Sprintf("**Cause**: %s\n\n", v.Cause))
			}

			if v.Details != "" {
				md.WriteString(fmt.Sprintf("%s\n\n", v.Details))
			}
		}
	} else if ruleInfo != nil && ruleInfo.IsAllowed {
		md.WriteString("## Allowed\n\n")
		md.WriteString(fmt.Sprintf("This import is allowed for files in `%s`\n\n", ruleInfo.Path))
	} else if ruleInfo != nil && !ruleInfo.IsAllowed {
		md.WriteString("## Prohibited\n\n")
		md.WriteString(fmt.Sprintf("This import is prohibited for files in `%s`\n\n", ruleInfo.Path))
		if ruleInfo.Reason != "" {
			md.WriteString(fmt.Sprintf("**Reason**: %s\n\n", ruleInfo.Reason))
		}
	} else {
		md.WriteString("## No Rules\n\n")
		md.WriteString("No architecture rules defined for this import.\n\n")
	}

	// Add config file location
	if ruleInfo != nil {
		md.WriteString("---\n\n")
		md.WriteString(fmt.Sprintf("**Rule Path**: `%s`\n\n", ruleInfo.Path))
		md.WriteString("*Defined in `.goverhaul.yml`*\n")
	}

	return MarkupContent{
		Kind:  Markdown,
		Value: md.String(),
	}
}

// getSeverityIcon returns an icon for the given severity
func getSeverityIcon(severity string) string {
	switch severity {
	case "error":
		return "❌"
	case "warning":
		return "⚠️"
	case "info":
		return "ℹ️"
	case "hint":
		return "💡"
	default:
		return "•"
	}
}

// uriToPath converts a file:// URI to a file path
func uriToPath(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("invalid URI: %w", err)
	}

	if u.Scheme != "file" {
		return "", fmt.Errorf("unsupported URI scheme: %s", u.Scheme)
	}

	// For file URIs, the path is in the Path component
	path := u.Path

	// On Windows, remove leading slash if it exists before drive letter
	if len(path) > 2 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}

	// Clean the path
	return filepath.Clean(path), nil
}
