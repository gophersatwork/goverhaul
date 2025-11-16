package lsp

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestConfigEditor_AddToAllowedList(t *testing.T) {
	tests := []struct {
		name         string
		initialYAML  string
		rulePath     string
		importPath   string
		checkResult  func(*testing.T, []TextEdit, afero.Fs)
	}{
		{
			name: "adds import to existing rule",
			initialYAML: `rules:
  - path: "internal/api"
    allowed:
      - "fmt"
      - "strings"
    prohibited:
      - name: "internal/database"
        cause: "Use repository pattern"
`,
			rulePath:   "internal/api",
			importPath: "encoding/json",
			checkResult: func(t *testing.T, edits []TextEdit, fs afero.Fs) {
				require.Len(t, edits, 1)

				// Parse the new content
				var config struct {
					Rules []struct {
						Path       string   `yaml:"path"`
						Allowed    []string `yaml:"allowed"`
					} `yaml:"rules"`
				}
				err := yaml.Unmarshal([]byte(edits[0].NewText), &config)
				require.NoError(t, err)

				// Check that the import was added
				require.Len(t, config.Rules, 1)
				assert.Contains(t, config.Rules[0].Allowed, "encoding/json")
				assert.Contains(t, config.Rules[0].Allowed, "fmt")
				assert.Contains(t, config.Rules[0].Allowed, "strings")
			},
		},
		{
			name: "creates allowed list if it doesn't exist",
			initialYAML: `rules:
  - path: "internal/api"
    prohibited:
      - name: "internal/database"
        cause: "Use repository pattern"
`,
			rulePath:   "internal/api",
			importPath: "fmt",
			checkResult: func(t *testing.T, edits []TextEdit, fs afero.Fs) {
				require.Len(t, edits, 1)

				var config struct {
					Rules []struct {
						Path    string   `yaml:"path"`
						Allowed []string `yaml:"allowed"`
					} `yaml:"rules"`
				}
				err := yaml.Unmarshal([]byte(edits[0].NewText), &config)
				require.NoError(t, err)

				require.Len(t, config.Rules, 1)
				assert.Contains(t, config.Rules[0].Allowed, "fmt")
			},
		},
		{
			name:        "creates rule if it doesn't exist",
			initialYAML: `rules: []`,
			rulePath:    "internal/api",
			importPath:  "fmt",
			checkResult: func(t *testing.T, edits []TextEdit, fs afero.Fs) {
				require.Len(t, edits, 1)

				var config struct {
					Rules []struct {
						Path    string   `yaml:"path"`
						Allowed []string `yaml:"allowed"`
					} `yaml:"rules"`
				}
				err := yaml.Unmarshal([]byte(edits[0].NewText), &config)
				require.NoError(t, err)

				require.Len(t, config.Rules, 1)
				assert.Equal(t, "internal/api", config.Rules[0].Path)
				assert.Contains(t, config.Rules[0].Allowed, "fmt")
			},
		},
		{
			name: "doesn't duplicate existing import",
			initialYAML: `rules:
  - path: "internal/api"
    allowed:
      - "fmt"
`,
			rulePath:   "internal/api",
			importPath: "fmt",
			checkResult: func(t *testing.T, edits []TextEdit, fs afero.Fs) {
				require.Len(t, edits, 1)

				var config struct {
					Rules []struct {
						Path    string   `yaml:"path"`
						Allowed []string `yaml:"allowed"`
					} `yaml:"rules"`
				}
				err := yaml.Unmarshal([]byte(edits[0].NewText), &config)
				require.NoError(t, err)

				// Should still have only one "fmt" entry
				require.Len(t, config.Rules, 1)
				count := 0
				for _, allowed := range config.Rules[0].Allowed {
					if allowed == "fmt" {
						count++
					}
				}
				assert.Equal(t, 1, count)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			configPath := "/test/.goverhaul.yml"

			err := afero.WriteFile(fs, configPath, []byte(tt.initialYAML), 0644)
			require.NoError(t, err)

			editor := NewConfigEditor(fs, configPath)
			edits, err := editor.AddToAllowedList(tt.rulePath, tt.importPath)

			require.NoError(t, err)
			if tt.checkResult != nil {
				tt.checkResult(t, edits, fs)
			}
		})
	}
}

func TestFindConfigFile(t *testing.T) {
	tests := []struct {
		name        string
		setupFS     func(afero.Fs)
		fileURI     string
		expectedCfg string
		expectErr   bool
	}{
		{
			name: "finds config in same directory",
			setupFS: func(fs afero.Fs) {
				afero.WriteFile(fs, "/project/main.go", []byte("package main"), 0644)
				afero.WriteFile(fs, "/project/.goverhaul.yml", []byte("rules: []"), 0644)
			},
			fileURI:     "file:///project/main.go",
			expectedCfg: "/project/.goverhaul.yml",
			expectErr:   false,
		},
		{
			name: "finds config in parent directory",
			setupFS: func(fs afero.Fs) {
				afero.WriteFile(fs, "/project/internal/api/handler.go", []byte("package api"), 0644)
				afero.WriteFile(fs, "/project/.goverhaul.yml", []byte("rules: []"), 0644)
			},
			fileURI:     "file:///project/internal/api/handler.go",
			expectedCfg: "/project/.goverhaul.yml",
			expectErr:   false,
		},
		{
			name: "prefers .goverhaul.yml over goverhaul.yml",
			setupFS: func(fs afero.Fs) {
				afero.WriteFile(fs, "/project/main.go", []byte("package main"), 0644)
				afero.WriteFile(fs, "/project/.goverhaul.yml", []byte("rules: []"), 0644)
				afero.WriteFile(fs, "/project/goverhaul.yml", []byte("other: []"), 0644)
			},
			fileURI:     "file:///project/main.go",
			expectedCfg: "/project/.goverhaul.yml",
			expectErr:   false,
		},
		{
			name: "returns error when no config found",
			setupFS: func(fs afero.Fs) {
				afero.WriteFile(fs, "/project/main.go", []byte("package main"), 0644)
			},
			fileURI:   "file:///project/main.go",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			tt.setupFS(fs)

			configPath, err := FindConfigFile(fs, tt.fileURI)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedCfg, configPath)
			}
		})
	}
}

func TestConfigEditor_findRulesNode(t *testing.T) {
	editor := &ConfigEditor{}

	// Test with a valid YAML structure
	yamlContent := `rules:
  - path: "internal/api"
    allowed:
      - "fmt"
`
	var node yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &node)
	require.NoError(t, err)

	rulesNode, err := editor.findRulesNode(&node)
	require.NoError(t, err)
	assert.NotNil(t, rulesNode)
	assert.Equal(t, yaml.SequenceNode, rulesNode.Kind)
}

func TestConfigEditor_findOrCreateRuleNode(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		rulePath     string
		shouldCreate bool
	}{
		{
			name: "finds existing rule",
			yamlContent: `rules:
  - path: "internal/api"
    allowed:
      - "fmt"
`,
			rulePath:     "internal/api",
			shouldCreate: false,
		},
		{
			name:         "creates new rule when not found",
			yamlContent:  `rules: []`,
			rulePath:     "internal/api",
			shouldCreate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			editor := &ConfigEditor{}

			var node yaml.Node
			err := yaml.Unmarshal([]byte(tt.yamlContent), &node)
			require.NoError(t, err)

			rulesNode, err := editor.findRulesNode(&node)
			require.NoError(t, err)

			initialLen := len(rulesNode.Content)

			ruleNode, err := editor.findOrCreateRuleNode(rulesNode, tt.rulePath)
			require.NoError(t, err)
			assert.NotNil(t, ruleNode)

			if tt.shouldCreate {
				assert.Greater(t, len(rulesNode.Content), initialLen)
			}
		})
	}
}
