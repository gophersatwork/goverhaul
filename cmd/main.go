package main

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/alexrios/goverhaul"
	"github.com/charmbracelet/fang"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	path    string
	verbose bool
)

func main() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.goverhaul/config.yml)")
	rootCmd.PersistentFlags().StringVar(&path, "path", ".", "path to lint")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose logging")

	// Execute the command and handle errors
	if err := fang.Execute(context.Background(), rootCmd); err != nil {
		logFile, logErr := setupLogFile()
		var logger *slog.Logger

		// Fall back to stderr if we can't create the log file
		if logErr != nil {
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelError,
			}))
			logger.Error("Failed to set up log file, falling back to stderr", "error", logErr)
		} else {
			defer logFile.Close()
			logger = slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
				Level: slog.LevelError,
			}))
		}

		// Check for specific error types
		info, found := goverhaul.GetErrorInfo(err)
		if found {
			logger.Error("Command failed",
				"error_type", info.Type)

			if info.Details != "" {
				logger.Error("Additional details", "details", info.Details)
			}

			if info.File != "" {
				logger.Error("File information", "file", info.File)
			}
		} else {
			var violations *goverhaul.LintViolations
			if errors.As(err, &violations) {
				logger.Error("Architecture rules violated",
					"message", "The codebase contains imports that violate the defined architectural rules",
					"details", violations.Error())
			} else if errors.Is(err, goverhaul.ErrLint) {
				logger.Error("Architecture rules violated",
					"message", "The codebase contains imports that violate the defined architectural rules")
			} else {
				logger.Error("Command failed", "error", err)
			}
		}

		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "goverhaul",
	Short: "A linter for Go architecture",
	Long:  `Goverhaul is a CLI tool to enforce architectural rules in Go projects.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize logger
		logLevel := slog.LevelInfo
		if verbose {
			logLevel = slog.LevelDebug
		}

		// Set up log file
		logFile, err := setupLogFile()
		if err != nil {
			// Fall back to stdout if we can't create the log file
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: logLevel,
			}))
			logger.Error("Failed to set up log file, falling back to stdout", "error", err)
			return err
		}
		defer logFile.Close()

		logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
			Level: logLevel,
		}))

		fs := afero.NewOsFs() // real fs binding

		cfg, err := goverhaul.LoadConfig(fs, cfgFile)
		if err != nil {
			logger.Error("Failed to load configuration", "error", err)
			return err
		}

		linter, err := goverhaul.NewLinter(cfg, logger, fs)
		if err != nil {
			logger.Error("Failed to initialize the linter", "error", err)
			return err
		}

		if err := linter.Lint(path); err != nil {
			// Handle different types of linting errors
			var violations *goverhaul.LintViolations
			if errors.As(err, &violations) {
				// For lint violations, display the detailed error message with file information
				logger.Error("Linting failed: architectural rules violated",
					"path", path,
					"details", violations.Error())
				return err
			}

			info, found := goverhaul.GetErrorInfo(err)
			if found {
				// For application errors, provide more context
				logger.Error("Linting failed",
					"error", err.Error(),
					"error_type", info.Type)

				if info.File != "" {
					logger.Info("File information", "file", info.File)
				}

				if info.Details != "" {
					logger.Info("Additional information", "details", info.Details)
				}
			} else {
				logger.Error("Linting failed with unexpected error", "error", err)
			}
			return err
		}

		return nil
	},
}

// setupLogFile creates the .goverhaul directory if it doesn't exist and returns a file handle for the log file
func setupLogFile() (*os.File, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// Create .goverhaul directory if it doesn't exist
	goverhaulDir := goverhaul.JoinPaths(home, ".goverhaul")
	if err := os.MkdirAll(goverhaulDir, 0o755); err != nil {
		return nil, err
	}

	// Open log file
	logFile := goverhaul.JoinPaths(goverhaulDir, "goverhaul.log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	return file, nil
}