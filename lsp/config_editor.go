package lsp

import (
	"fmt"
	"strings"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

// ConfigEditor modifies .goverhaul.yml programmatically
type ConfigEditor struct {
	fs   afero.Fs
	path string
}

// NewConfigEditor creates a new ConfigEditor
func NewConfigEditor(fs afero.Fs, path string) *ConfigEditor {
	return &ConfigEditor{
		fs:   fs,
		path: path,
	}
}

// AddToAllowedList adds an import to the allowed list for a specific rule path
func (e *ConfigEditor) AddToAllowedList(rulePath string, importPath string) ([]TextEdit, error) {
	// Read the config file
	content, err := afero.ReadFile(e.fs, e.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML into a node tree to preserve structure and comments
	var node yaml.Node
	if err := yaml.Unmarshal(content, &node); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	// Find the rules node
	rulesNode, err := e.findRulesNode(&node)
	if err != nil {
		return nil, err
	}

	// Find or create the rule with the specified path
	ruleNode, err := e.findOrCreateRuleNode(rulesNode, rulePath)
	if err != nil {
		return nil, err
	}

	// Add the import to the allowed list
	if err := e.addToAllowed(ruleNode, importPath); err != nil {
		return nil, err
	}

	// Marshal back to YAML
	newContent, err := yaml.Marshal(&node)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated config: %w", err)
	}

	// Create a text edit that replaces the entire file
	// In a real LSP implementation, we might want to be more surgical
	edits := []TextEdit{
		{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: countLines(string(content)), Character: 0},
			},
			NewText: string(newContent),
		},
	}

	return edits, nil
}

// findRulesNode finds the "rules" node in the YAML document
func (e *ConfigEditor) findRulesNode(root *yaml.Node) (*yaml.Node, error) {
	// Root is typically a document node
	if root.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("expected document node")
	}

	// The document content should be a mapping node
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected mapping node in document")
	}

	mapping := root.Content[0]

	// Find "rules" key in the mapping
	for i := 0; i < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		if key.Value == "rules" {
			return mapping.Content[i+1], nil
		}
	}

	// Rules not found, create it
	rulesKey := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "rules",
	}
	rulesValue := &yaml.Node{
		Kind: yaml.SequenceNode,
	}

	mapping.Content = append(mapping.Content, rulesKey, rulesValue)
	return rulesValue, nil
}

// findOrCreateRuleNode finds a rule with the given path or creates it
func (e *ConfigEditor) findOrCreateRuleNode(rulesNode *yaml.Node, rulePath string) (*yaml.Node, error) {
	if rulesNode.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("rules node is not a sequence")
	}

	// Search for existing rule
	for _, ruleNode := range rulesNode.Content {
		if ruleNode.Kind != yaml.MappingNode {
			continue
		}

		// Find the "path" field
		for i := 0; i < len(ruleNode.Content); i += 2 {
			key := ruleNode.Content[i]
			value := ruleNode.Content[i+1]
			if key.Value == "path" && value.Value == rulePath {
				return ruleNode, nil
			}
		}
	}

	// Rule not found, create new one
	newRule := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "path"},
			{Kind: yaml.ScalarNode, Value: rulePath},
		},
	}

	rulesNode.Content = append(rulesNode.Content, newRule)
	return newRule, nil
}

// addToAllowed adds an import to the allowed list of a rule
func (e *ConfigEditor) addToAllowed(ruleNode *yaml.Node, importPath string) error {
	if ruleNode.Kind != yaml.MappingNode {
		return fmt.Errorf("rule node is not a mapping")
	}

	// Find the "allowed" field
	for i := 0; i < len(ruleNode.Content); i += 2 {
		key := ruleNode.Content[i]
		if key.Value == "allowed" {
			allowedNode := ruleNode.Content[i+1]

			// Check if import already exists
			if allowedNode.Kind == yaml.SequenceNode {
				for _, item := range allowedNode.Content {
					if item.Value == importPath {
						// Already in the list
						return nil
					}
				}

				// Add the import
				newItem := &yaml.Node{
					Kind:  yaml.ScalarNode,
					Value: importPath,
				}
				allowedNode.Content = append(allowedNode.Content, newItem)
				return nil
			}
		}
	}

	// "allowed" field not found, create it
	allowedKey := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: "allowed",
	}
	allowedValue := &yaml.Node{
		Kind: yaml.SequenceNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: importPath},
		},
	}

	ruleNode.Content = append(ruleNode.Content, allowedKey, allowedValue)
	return nil
}

// countLines counts the number of lines in a string
func countLines(s string) int {
	return strings.Count(s, "\n")
}

// FindConfigFile searches for a config file starting from the given URI
func FindConfigFile(fs afero.Fs, fileURI string) (string, error) {
	// Convert file:// URI to path
	path := strings.TrimPrefix(fileURI, "file://")

	// Common config file names
	configNames := []string{".goverhaul.yml", "goverhaul.yml", ".goverhaul.yaml", "goverhaul.yaml"}

	// Start from the directory of the file and walk up
	dir := path
	if !isDirFs(fs, dir) {
		dir = dirPath(dir)
	}

	for {
		for _, name := range configNames {
			configPath := joinPaths(dir, name)
			exists, _ := afero.Exists(fs, configPath)
			if exists {
				return configPath, nil
			}
		}

		// Move up one directory
		parent := dirPath(dir)
		if parent == dir || parent == "" {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("config file not found")
}

// Helper functions (these would typically come from a path utility package)
func isDirFs(fs afero.Fs, path string) bool {
	info, err := fs.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func dirPath(path string) string {
	// Simple implementation - in production, use filepath.Dir
	idx := strings.LastIndex(path, "/")
	if idx == -1 {
		return ""
	}
	return path[:idx]
}

func joinPaths(parts ...string) string {
	// Simple implementation - in production, use filepath.Join
	return strings.Join(parts, "/")
}
