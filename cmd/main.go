package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/gophersatwork/goverhaul"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var (
	cfgFile     string
	path        string
	verbose     bool
	groupByRule bool
)

func main() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")
	rootCmd.PersistentFlags().StringVar(&path, "path", ".", "path to lint")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose logging")
	rootCmd.PersistentFlags().BoolVar(&groupByRule, "group-by-rule", false, "group violations by rule instead of by file")

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

		// Check for specific error details
		appErr, found := goverhaul.GetErrorInfo(err)
		if found {
			if appErr.Details != "" {
				logger.Error("Additional details", "details", appErr.Details)
			}

			if appErr.File != "" {
				logger.Error("File information", "file", appErr.File)
			}
		}
		logger.Error("Command failed", "error", err)
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

		// Set up logger
		var logger *slog.Logger
		if verbose {
			// When verbose is true, log to stdout for better visibility
			logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: logLevel,
			}))
		} else {
			// Otherwise, log to file
			logFile, err := setupLogFile()
			if err != nil {
				// Fall back to stdout if we can't create the log file
				logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
					Level: logLevel,
				}))
				logger.Error("Failed to set up log file, falling back to stdout", "error", err)
				return err
			}
			defer logFile.Close()

			logger = slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
				Level: logLevel,
			}))
		}

		fs := afero.NewOsFs() // real fs binding
		cfg, err := goverhaul.LoadConfig(fs, path, cfgFile)
		if err != nil {
			logger.Error("Failed to load configuration", "error", err)
			return err
		}

		linter, err := goverhaul.NewLinter(cfg, logger, fs)
		if err != nil {
			logger.Error("Failed to initialize the linter", "error", err)
			return err
		}

		lv, err := linter.Lint(path)
		if err != nil {
			return err
		}

		if groupByRule {
			fmt.Println(lv.PrintByRule())
		} else {
			fmt.Println(lv.PrintByFile())
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
