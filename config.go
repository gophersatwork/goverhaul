package goverhaul

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/viper"
)

type Config struct {
	Rules       []Rule `yaml:"rules" mapstructure:"rules"`
	Modfile     string `yaml:"modfile" mapstructure:"modfile"`
	Incremental bool   `yaml:"incremental" mapstructure:"incremental"`
	CacheFile   string `yaml:"cache_file" mapstructure:"cache_file"`
}

type Rule struct {
	Path       string          `yaml:"path" mapstructure:"path"`
	Allowed    []string        `yaml:"allowed" mapstructure:"allowed"`
	Prohibited []ProhibitedPkg `yaml:"prohibited" mapstructure:"prohibited"`
}

type ProhibitedPkg struct {
	Name  string `yaml:"name" mapstructure:"name"`
	Cause string `yaml:"cause" mapstructure:"cause"`
}

func LoadConfig(fs afero.Fs, path string, cfgFile string) (Config, error) {
	viper.SetFs(fs)
	viper.SetConfigType("yml") // Always set the config type to yml

	// Check if cfgFile is a full path to a file
	fileInfo, statErr := fs.Stat(cfgFile)
	if statErr == nil && !fileInfo.IsDir() {
		// cfgFile is a full path to an existing file
		viper.SetConfigFile(cfgFile)
	} else {
		// Use the provided config file or default to config.yml
		if cfgFile != "" {
			// Handle case where cfgFile includes extension
			if strings.HasSuffix(cfgFile, ".yml") || strings.HasSuffix(cfgFile, ".yaml") {
				viper.SetConfigFile(cfgFile)
			} else {
				viper.SetConfigName(cfgFile)
			}
		} else {
			viper.SetConfigName("config")
		}

		viper.AddConfigPath(path)
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.goverhaul")
		viper.AddConfigPath("./.goverhaul")
	}

	if err := viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			return Config{}, NewConfigError("config file not found", err)
		} else {
			return Config{}, NewConfigError("failed loading config file", err)
		}
	}

	viper.SetDefault("incremental", false)
	viper.SetDefault("rules", []Rule{})
	viper.SetDefault("modfile", "go.mod")
	viper.SetDefault("cache_file", "cache.json")

	var config Config
	err := viper.Unmarshal(&config)
	if err != nil {
		return Config{}, NewConfigError("failed unmarshaling config file", err)
	}

	return config, nil
}

func printDirTree(fs afero.Fs, path string) error {
	err := afero.Walk(fs, path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if p == path {
			return nil
		}

		depth := strings.Count(p, string(os.PathSeparator))
		indent := strings.Repeat("│   ", depth-1)

		name := info.Name()
		if info.IsDir() {
			fmt.Printf("%s├── 📁 %s\n", indent, name)
		} else {
			fmt.Printf("%s├── 📄 %s\n", indent, name)
		}

		return nil
	})
	if err != nil {
		log.Fatalf("Failed to inspect the folder: %v", err)
	}

	return nil
}
