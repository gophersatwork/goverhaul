package lsp

import (
	"fmt"
	"strings"

	"github.com/gophersatwork/goverhaul"
	"github.com/spf13/afero"
)

// CodeActionProvider generates quick fixes for violations
type CodeActionProvider struct {
	fs         afero.Fs
	configPath string
}

// NewCodeActionProvider creates a new code action provider
func NewCodeActionProvider(fs afero.Fs, configPath string) *CodeActionProvider {
	return &CodeActionProvider{
		fs:         fs,
		configPath: configPath,
	}
}

// GetCodeActions returns available quick fixes for a diagnostic
func (p *CodeActionProvider) GetCodeActions(
	uri string,
	diagnostic Diagnostic,
) []CodeAction {
	var actions []CodeAction

	// Only process goverhaul diagnostics
	if diagnostic.Source != "goverhaul" {
		return actions
	}

	// Extract the import path from the diagnostic message
	importPath := ExtractImportPath(diagnostic.Message)
	if importPath == "" {
		return actions
	}

	// Convert URI to file path
	filePath := strings.TrimPrefix(uri, "file://")

	// 1. "Remove import" action
	removeAction := p.createRemoveImportAction(uri, filePath, diagnostic, importPath)
	if removeAction != nil {
		actions = append(actions, *removeAction)
	}

	// 2. "Add to allowed list" action
	allowedAction := p.createAddToAllowedAction(uri, filePath, diagnostic, importPath)
	if allowedAction != nil {
		actions = append(actions, *allowedAction)
	}

	// 3. "Suppress violation" action (add comment)
	suppressAction := p.createSuppressAction(uri, filePath, diagnostic, importPath)
	if suppressAction != nil {
		actions = append(actions, *suppressAction)
	}

	return actions
}

// createRemoveImportAction creates an action to remove the import statement
func (p *CodeActionProvider) createRemoveImportAction(
	uri string,
	filePath string,
	diag Diagnostic,
	importPath string,
) *CodeAction {
	// Get the exact range of the import including the entire line
	importRange, _, err := GetImportBlockRange(p.fs, filePath, importPath)
	if err != nil {
		return nil
	}

	return &CodeAction{
		Title: fmt.Sprintf("Remove import \"%s\"", importPath),
		Kind:  QuickFix,
		Diagnostics: []Diagnostic{diag},
		Edit: &WorkspaceEdit{
			Changes: map[string][]TextEdit{
				uri: {
					{
						Range:   importRange,
						NewText: "", // Delete the import line
					},
				},
			},
		},
		IsPreferred: false,
	}
}

// createAddToAllowedAction creates an action to add the import to the allowed list
func (p *CodeActionProvider) createAddToAllowedAction(
	uri string,
	filePath string,
	diag Diagnostic,
	importPath string,
) *CodeAction {
	// Find the config file
	configPath := p.configPath
	if configPath == "" {
		var err error
		configPath, err = FindConfigFile(p.fs, uri)
		if err != nil {
			// Can't find config, can't provide this action
			return nil
		}
	}

	// Load config to determine the rule path
	cfg, err := goverhaul.LoadConfig(p.fs, dirPath(filePath), configPath)
	if err != nil {
		return nil
	}

	// Find which rule this file falls under
	rulePath := p.findApplicableRule(cfg, filePath)
	if rulePath == "" {
		// No applicable rule found
		return nil
	}

	// Create config editor
	editor := NewConfigEditor(p.fs, configPath)

	// Generate the edit to add import to allowed list
	edits, err := editor.AddToAllowedList(rulePath, importPath)
	if err != nil {
		return nil
	}

	// Convert file path to URI
	configURI := "file://" + configPath

	return &CodeAction{
		Title: fmt.Sprintf("Add \"%s\" to allowed list", importPath),
		Kind:  QuickFix,
		Diagnostics: []Diagnostic{diag},
		Edit: &WorkspaceEdit{
			Changes: map[string][]TextEdit{
				configURI: edits,
			},
		},
		IsPreferred: true,
	}
}

// createSuppressAction creates an action to suppress the violation with a comment
func (p *CodeActionProvider) createSuppressAction(
	uri string,
	filePath string,
	diag Diagnostic,
	importPath string,
) *CodeAction {
	// Get the line above the import
	lineAbove, err := GetLineAboveImport(p.fs, filePath, importPath)
	if err != nil {
		return nil
	}

	return &CodeAction{
		Title: "Suppress this violation",
		Kind:  QuickFix,
		Diagnostics: []Diagnostic{diag},
		Edit: &WorkspaceEdit{
			Changes: map[string][]TextEdit{
				uri: {
					{
						Range: Range{
							Start: Position{Line: lineAbove, Character: 0},
							End:   Position{Line: lineAbove, Character: 0},
						},
						NewText: "// goverhaul:ignore - Legacy code, will refactor\n",
					},
				},
			},
		},
		IsPreferred: false,
	}
}

// findApplicableRule finds the rule path that applies to the given file
func (p *CodeActionProvider) findApplicableRule(cfg goverhaul.Config, filePath string) string {
	for _, rule := range cfg.Rules {
		if p.ruleAppliesToPath(rule, filePath) {
			return rule.Path
		}
	}
	return ""
}

// ruleAppliesToPath checks if a rule applies to a file
// This is a simplified version - in production, use the same logic as the linter
func (p *CodeActionProvider) ruleAppliesToPath(rule goverhaul.Rule, filePath string) bool {
	// Normalize paths
	rulePath := normalizePath(rule.Path)
	fileDir := dirPath(filePath)

	// Check if file is in the rule's path
	return strings.HasPrefix(fileDir, rulePath) || strings.Contains(fileDir, "/"+rulePath)
}

// normalizePath normalizes a file path
func normalizePath(path string) string {
	return strings.TrimPrefix(strings.TrimPrefix(path, "./"), "/")
}
