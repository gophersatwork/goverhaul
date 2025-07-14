package goverhaul

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tests := map[string]struct {
		setupConfigFile func(fs afero.Fs) error
	}{
		"should load config from the current directory": {
			setupConfigFile: func(fs afero.Fs) error {
				return afero.WriteFile(fs, "config", defaultConfigTestFile(t), 0o644)
			},
		},
		"should load config from .goverhaul folder in the current directory": {
			setupConfigFile: func(fs afero.Fs) error {
				err := fs.Mkdir(".goverhaul", 0o755)
				if err != nil {
					return err
				}
				return afero.WriteFile(fs, ".goverhaul/config.yml", defaultConfigTestFile(t), 0o644)
			},
		},
		"should load config from /home/test/.goverhaul directory": {
			setupConfigFile: func(fs afero.Fs) error {
				err := fs.Mkdir("/home/test/.goverhaul", 0o755)
				if err != nil {
					return err
				}
				return afero.WriteFile(fs, "/home/test/.goverhaul/config", defaultConfigTestFile(t), 0o644)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			memFs := afero.NewMemMapFs()
			// simulate home dir for a 'test' user
			err := test.setupConfigFile(memFs)
			if err != nil {
				t.Fatal(err)
			}

			// Set the path and config file name based on the test case
			path := "."
			cfgFile := "config"
			if name == "should load config from .goverhaul folder in the current directory" {
				// For this test, use the full path to the config file
				cfgFile = ".goverhaul/config.yml"
			} else if name == "should load config from /home/test/.goverhaul directory" {
				path = "/home/test/.goverhaul"
			}

			config, err := LoadConfig(memFs, path, cfgFile)
			if err != nil {
				t.Fatalf("failed to load config: %v", err)
			}

			assertDefaultConfigTestFile(t, config)
		})
	}
}

func TestEmptyConfig(t *testing.T) {
	memFs := afero.NewMemMapFs()

	var emptyContent []byte
	afero.WriteFile(memFs, "config", emptyContent, 0o644)

	config, err := LoadConfig(memFs, ".", "config")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	assertDefaultValues(t, config)
}

func TestInvalidYamlConfig(t *testing.T) {
	memFs := afero.NewMemMapFs()

	invalidYAML := `
	rules:
	 - path: "internal"
	   allowed:
	     - "fmt"
	     - "errors
	   prohibited:
	     - "unsafe"
	`

	afero.WriteFile(memFs, "config", []byte(invalidYAML), 0o644)
	_, err := LoadConfig(memFs, ".", "config")
	require.Error(t, err)
	require.Equal(t, "[config] failed loading config file: While parsing config: yaml: line 2: found character that cannot start any token", err.Error())
}

func defaultConfigTestFile(t *testing.T) []byte {
	t.Helper()

	return []byte(`
rules:
  - path: "internal"
    allowed:
      - "fmt"
      - "errors"
    prohibited:
      - name: "unsafe"
        cause: "unsafe code is not allowed in internal packages"
  - path: "cmd"
    prohibited:
      - name: "internal/private"
        cause: "private internal packages should not be used directly"
modfile: "go.mod"
cache_file: "new_cache.json"
`)
}

func assertDefaultConfigTestFile(t *testing.T, config Config) {
	t.Helper()

	assert.Equal(t, "go.mod", config.Modfile)
	assert.False(t, config.Incremental)
	assert.Equal(t, "new_cache.json", config.CacheFile)

	assert.Equal(t, "internal", config.Rules[0].Path)
	assert.EqualValues(t, []string{"fmt", "errors"}, config.Rules[0].Allowed)
	assert.Equal(t, []ProhibitedPkg{{
		Name:  "unsafe",
		Cause: "unsafe code is not allowed in internal packages",
	}}, config.Rules[0].Prohibited)

	assert.Equal(t, "cmd", config.Rules[1].Path)
	assert.Empty(t, config.Rules[1].Allowed)
	assert.Equal(t, []ProhibitedPkg{{
		Name:  "internal/private",
		Cause: "private internal packages should not be used directly",
	}}, config.Rules[1].Prohibited)
}

func assertDefaultValues(t *testing.T, config Config) {
	t.Helper()

	assert.Equal(t, "go.mod", config.Modfile)
	assert.False(t, config.Incremental)
	assert.Equal(t, "cache.json", config.CacheFile)

	assert.Empty(t, config.Rules)
}
