package lsp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"

	"github.com/spf13/afero"
)

// ExtractImportPath parses import path from diagnostic message
// Expected format: "import \"internal/database\" violates..."
func ExtractImportPath(message string) string {
	// Match quoted import paths in the message
	re := regexp.MustCompile(`import\s+"([^"]+)"`)
	matches := re.FindStringSubmatch(message)
	if len(matches) > 1 {
		return matches[1]
	}

	// Fallback: try to extract any quoted string
	re = regexp.MustCompile(`"([^"]+)"`)
	matches = re.FindStringSubmatch(message)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// GetImportLineRange finds the exact range of an import statement
func GetImportLineRange(fs afero.Fs, filePath string, importPath string) (Range, error) {
	content, err := afero.ReadFile(fs, filePath)
	if err != nil {
		return Range{}, err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, content, parser.ImportsOnly)
	if err != nil {
		return Range{}, err
	}

	// Find the import spec matching the import path
	for _, imp := range file.Imports {
		impPath := strings.Trim(imp.Path.Value, `"`)
		if impPath == importPath {
			pos := fset.Position(imp.Pos())
			end := fset.Position(imp.End())

			return Range{
				Start: Position{Line: pos.Line - 1, Character: pos.Column - 1},
				End:   Position{Line: end.Line - 1, Character: end.Column - 1},
			}, nil
		}
	}

	// If not found, return zero range
	return Range{}, nil
}

// GetImportBlockRange returns the range of the entire import declaration
// including the import keyword and parentheses if it's a grouped import
func GetImportBlockRange(fs afero.Fs, filePath string, importPath string) (Range, bool, error) {
	content, err := afero.ReadFile(fs, filePath)
	if err != nil {
		return Range{}, false, err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, content, parser.ImportsOnly)
	if err != nil {
		return Range{}, false, err
	}

	// Find the import spec and its parent declaration
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.IMPORT {
			continue
		}

		// Check if this declaration contains our import
		for _, spec := range genDecl.Specs {
			impSpec, ok := spec.(*ast.ImportSpec)
			if !ok {
				continue
			}

			impPath := strings.Trim(impSpec.Path.Value, `"`)
			if impPath == importPath {
				// Check if this is a single import or part of a group
				isSingleImport := len(genDecl.Specs) == 1

				if isSingleImport {
					// For single imports, return the entire line including "import"
					pos := fset.Position(genDecl.Pos())
					end := fset.Position(genDecl.End())

					// Include the newline after the import
					return Range{
						Start: Position{Line: pos.Line - 1, Character: 0},
						End:   Position{Line: end.Line, Character: 0},
					}, true, nil
				} else {
					// For grouped imports, return only the import line
					pos := fset.Position(impSpec.Pos())
					end := fset.Position(impSpec.End())

					// Include the newline and any indentation
					return Range{
						Start: Position{Line: pos.Line - 1, Character: 0},
						End:   Position{Line: end.Line, Character: 0},
					}, false, nil
				}
			}
		}
	}

	return Range{}, false, nil
}

// GetLineAboveImport returns the line number just above an import statement
func GetLineAboveImport(fs afero.Fs, filePath string, importPath string) (int, error) {
	importRange, _, err := GetImportBlockRange(fs, filePath, importPath)
	if err != nil {
		return 0, err
	}

	return importRange.Start.Line, nil
}
