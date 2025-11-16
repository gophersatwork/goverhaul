package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

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
	colorMode   string
)

func main() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")
	rootCmd.PersistentFlags().StringVar(&path, "path", ".", "path to lint")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose logging")
	rootCmd.PersistentFlags().BoolVar(&groupByRule, "group-by-rule", false, "group violations by rule instead of by file")
	rootCmd.PersistentFlags().StringVar(&colorMode, "color", "auto", "when to use colors: auto, always, never")

	// Add watch command
	rootCmd.AddCommand(watchCmd)

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

		// Create text formatter with color and grouping options
		formatter := goverhaul.NewTextFormatter()
		formatter.GroupByRule = groupByRule

		// Parse color mode
		switch colorMode {
		case "always":
			formatter.ColorMode = goverhaul.ColorAlways
		case "never":
			formatter.ColorMode = goverhaul.ColorNever
		case "auto":
			formatter.ColorMode = goverhaul.ColorAuto
		default:
			logger.Warn("Invalid color mode, defaulting to auto", "mode", colorMode)
			formatter.ColorMode = goverhaul.ColorAuto
		}

		// Format and print violations
		output, err := formatter.Format(lv, &cfg)
		if err != nil {
			return err
		}

		fmt.Print(string(output))

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

// watchCmd represents the watch command
var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Continuously watch for file changes and re-lint",
	Long: `Watch mode continuously monitors your Go files for changes and automatically
re-runs the linter when changes are detected. It uses intelligent debouncing
to group rapid changes together and leverages the MUS cache for instant feedback.

Features:
  - Real-time file monitoring with fsnotify
  - Debounced change detection (100ms window)
  - Incremental analysis for changed files only
  - Config hot-reload on configuration changes
  - Beautiful colored output with progress indicators

Example:
  # Watch current directory
  goverhaul watch

  # Watch specific path
  goverhaul watch --path ./internal

  # Custom config
  goverhaul watch --config .goverhaul.yml`,
	RunE: runWatch,
}

func runWatch(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}

	// Set up logger
	var logger *slog.Logger
	if verbose {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: logLevel,
		}))
	} else {
		logFile, err := setupLogFile()
		if err != nil {
			logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: logLevel,
			}))
			logger.Error("Failed to set up log file, falling back to stdout", "error", err)
		} else {
			defer logFile.Close()
			logger = slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
				Level: logLevel,
			}))
		}
	}

	// Create watch mode
	fs := afero.NewOsFs()
	watchMode, err := goverhaul.NewWatchMode(goverhaul.WatchConfig{
		Path:        path,
		ConfigPath:  cfgFile,
		Logger:      logger,
		FS:          fs,
		GroupByRule: groupByRule,
		ColorMode:   colorMode,
	})
	if err != nil {
		return fmt.Errorf("failed to create watch mode: %w", err)
	}
	defer watchMode.Stop()

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start watching in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- watchMode.Start(ctx, path)
	}()

	// Wait for either completion or signal
	select {
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("watch mode error: %w", err)
		}
		return nil
	case <-sigChan:
		logger.Info("Received interrupt signal, shutting down gracefully...")
		cancel()
		// Wait for watch mode to finish
		return <-errChan
	}
}
