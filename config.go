package goverhaul

import (
	"errors"

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

func LoadConfig(fs afero.Fs, cfgFile string) (Config, error) {
	viper.SetFs(fs)
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.goverhaul")
	viper.AddConfigPath("./.goverhaul")

	// Use the provided config file or default to config.yml
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigFile("config.yml")
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
