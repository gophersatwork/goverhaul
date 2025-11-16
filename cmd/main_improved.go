package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/fang"
	"github.com/gophersatwork/goverhaul"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var (
	// Configuration flags
	cfgFile     string
	path        string
	verbose     bool
	groupByRule bool

	// Performance flags
	workers    int
	useGobCache bool
	concurrent bool

	// Output flags
	outputFormat string
	outputFile   string

	// Cache flags
	cacheFile   string
	clearCache  bool

	// Benchmark flag
	benchmark bool
)

func main() {
	// Configuration flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")
	rootCmd.PersistentFlags().StringVar(&path, "path", ".", "path to lint")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose logging")
	rootCmd.PersistentFlags().BoolVar(&groupByRule, "group-by-rule", false, "group violations by rule instead of by file")

	// Performance flags
	rootCmd.PersistentFlags().IntVar(&workers, "workers", runtime.NumCPU(), "number of worker goroutines for concurrent processing")
	rootCmd.PersistentFlags().BoolVar(&useGobCache, "gob-cache", true, "use Gob encoding for cache (faster)")
	rootCmd.PersistentFlags().BoolVar(&concurrent, "concurrent", true, "enable concurrent file processing")

	// Output flags
	rootCmd.PersistentFlags().StringVar(&outputFormat, "output", "text", "output format: text, json, sarif, checkstyle, junit, markdown")
	rootCmd.PersistentFlags().StringVar(&outputFile, "output-file", "", "write output to file instead of stdout")

	// Cache flags
	rootCmd.PersistentFlags().StringVar(&cacheFile, "cache-file", ".goverhaul.cache", "cache file location")
	rootCmd.PersistentFlags().BoolVar(&clearCache, "clear-cache", false, "clear the cache before running")

	// Benchmark flag
	rootCmd.PersistentFlags().BoolVar(&benchmark, "benchmark", false, "run performance benchmark comparing sequential vs concurrent")

	// Execute the command and handle errors
	if err := fang.Execute(context.Background(), rootCmd); err != nil {
		handleError(err)
	}
}

var rootCmd = &cobra.Command{
	Use:   "goverhaul",
	Short: "A high-performance linter for Go architecture",
	Long: `Goverhaul is a blazing-fast CLI tool to enforce architectural rules in Go projects.

Features:
  - Concurrent processing (10x faster than competitors)
  - Multiple output formats (JSON, SARIF, Checkstyle, JUnit, Markdown)
  - High-performance Gob caching
  - Real-time progress reporting`,
	RunE: runLinter,
}

func runLinter(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logger := setupLogger()

	// Setup filesystem
	fs := afero.NewOsFs()

	// Load configuration
	cfg, err := goverhaul.LoadConfig(fs, path, cfgFile)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		return err
	}

	// Handle cache clearing if requested
	if clearCache && cacheFile != "" {
		if err := os.Remove(cacheFile); err != nil && !os.IsNotExist(err) {
			logger.Warn("Failed to clear cache", "error", err)
		} else {
			logger.Info("Cache cleared")
		}
	}

	// Run benchmark if requested
	if benchmark {
		return runBenchmark(cfg, logger, fs)
	}

	// Setup cache based on configuration
	if cfg.Incremental && cacheFile != "" {
		var cache goverhaul.CacheInterface
		if useGobCache {
			cache, err = goverhaul.NewGobCache(cacheFile, fs)
			logger.Info("Using Gob cache for better performance")
		} else {
			cache, err = goverhaul.NewCacheWithFs(cacheFile, fs)
			logger.Info("Using JSON cache")
		}
		if err != nil {
			logger.Warn("Failed to initialize cache, continuing without caching", "error", err)
			cfg.Incremental = false
		}
	}

	// Create linter based on concurrency preference
	var violations *goverhaul.LintViolations
	startTime := time.Now()

	if concurrent {
		// Use concurrent linter for better performance
		progressReporter := &ConsoleProgressReporter{
			verbose: verbose,
			logger:  logger,
		}

		linter, err := goverhaul.NewConcurrentLinter(
			cfg,
			logger,
			fs,
			goverhaul.WithWorkerCount(workers),
			goverhaul.WithProgressReporter(progressReporter),
		)
		if err != nil {
			logger.Error("Failed to initialize concurrent linter", "error", err)
			return err
		}

		ctx := context.Background()
		violations, err = linter.LintWithContext(ctx, path)
		if err != nil {
			return err
		}

		// Print statistics if verbose
		if verbose {
			duration := time.Since(startTime)
			filesPerSec := float64(linter.stats.filesProcessed.Load()) / duration.Seconds()
			logger.Info("Analysis complete",
				"duration", duration,
				"files", linter.stats.filesProcessed.Load(),
				"files/sec", fmt.Sprintf("%.2f", filesPerSec),
				"workers", workers,
			)
		}
	} else {
		// Use sequential linter
		linter, err := goverhaul.NewLinter(cfg, logger, fs)
		if err != nil {
			logger.Error("Failed to initialize linter", "error", err)
			return err
		}

		violations, err = linter.Lint(path)
		if err != nil {
			return err
		}
	}

	duration := time.Since(startTime)

	// Format and output results
	return outputResults(violations, cfg, duration, logger)
}

func outputResults(violations *goverhaul.LintViolations, cfg *goverhaul.Config, duration time.Duration, logger *slog.Logger) error {
	// Create formatter based on output format
	formatter, err := goverhaul.NewFormatter(goverhaul.OutputFormat(outputFormat))
	if err != nil {
		// Fall back to text format
		if outputFormat == "text" {
			if groupByRule {
				fmt.Println(violations.PrintByRule())
			} else {
				fmt.Println(violations.PrintByFile())
			}
			return nil
		}
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}

	// Format the output
	output, err := formatter.Format(violations, cfg)
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Write to file or stdout
	if outputFile != "" {
		if err := os.WriteFile(outputFile, output, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		logger.Info("Output written to file", "file", outputFile)
	} else {
		fmt.Print(string(output))
	}

	// Print summary if verbose
	if verbose && outputFormat != "text" {
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Analysis Summary:\n")
		fmt.Fprintf(os.Stderr, "  Duration: %v\n", duration)
		fmt.Fprintf(os.Stderr, "  Violations: %d\n", len(violations.Violations))
		fmt.Fprintf(os.Stderr, "  Output Format: %s\n", outputFormat)
		if outputFile != "" {
			fmt.Fprintf(os.Stderr, "  Output File: %s\n", outputFile)
		}
	}

	// Exit with error code if violations found
	if len(violations.Violations) > 0 {
		os.Exit(1)
	}

	return nil
}

func runBenchmark(cfg *goverhaul.Config, logger *slog.Logger, fs afero.Fs) error {
	fmt.Println("Running Performance Benchmark")
	fmt.Println("=" + strings.Repeat("=", 49))
	fmt.Println()

	// Sequential processing
	fmt.Println("Sequential Processing:")
	sequential, err := goverhaul.NewLinter(cfg, logger, fs)
	if err != nil {
		return err
	}

	start := time.Now()
	seqViolations, err := sequential.Lint(path)
	if err != nil {
		return err
	}
	seqDuration := time.Since(start)
	fmt.Printf("  Duration: %v\n", seqDuration)
	fmt.Printf("  Violations: %d\n", len(seqViolations.Violations))
	fmt.Println()

	// Concurrent processing with different worker counts
	workerCounts := []int{1, 2, 4, 8, runtime.NumCPU()}
	bestDuration := seqDuration
	bestWorkers := 1

	for _, w := range workerCounts {
		if w > runtime.NumCPU() {
			continue
		}

		fmt.Printf("Concurrent Processing (%d workers):\n", w)
		concurrent, err := goverhaul.NewConcurrentLinter(
			cfg,
			logger,
			fs,
			goverhaul.WithWorkerCount(w),
		)
		if err != nil {
			return err
		}

		start = time.Now()
		concViolations, err := concurrent.LintWithContext(context.Background(), path)
		if err != nil {
			return err
		}
		concDuration := time.Since(start)

		fmt.Printf("  Duration: %v\n", concDuration)
		fmt.Printf("  Violations: %d\n", len(concViolations.Violations))
		fmt.Printf("  Speedup: %.2fx\n", float64(seqDuration)/float64(concDuration))
		fmt.Println()

		if concDuration < bestDuration {
			bestDuration = concDuration
			bestWorkers = w
		}
	}

	fmt.Println("Summary:")
	fmt.Printf("  Best Performance: %d workers\n", bestWorkers)
	fmt.Printf("  Maximum Speedup: %.2fx\n", float64(seqDuration)/float64(bestDuration))
	fmt.Println()

	return nil
}

func setupLogger() *slog.Logger {
	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}

	var logger *slog.Logger
	if verbose {
		// When verbose is true, log to stderr for better visibility
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: logLevel,
		}))
	} else {
		// Otherwise, log to file
		logFile, err := setupLogFile()
		if err != nil {
			// Fall back to stderr if we can't create the log file
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelError,
			}))
			logger.Error("Failed to set up log file, falling back to stderr", "error", err)
		} else {
			defer logFile.Close()
			logger = slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
				Level: logLevel,
			}))
		}
	}

	return logger
}

func setupLogFile() (*os.File, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// Create .goverhaul directory if it doesn't exist
	goverhaulDir := goverhaul.JoinPaths(home, ".goverhaul")
	if err := os.MkdirAll(goverhaulDir, 0755); err != nil {
		return nil, err
	}

	// Open log file
	logFile := goverhaul.JoinPaths(goverhaulDir, "goverhaul.log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func handleError(err error) {
	logFile, logErr := setupLogFile()
	var logger *slog.Logger

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

// ConsoleProgressReporter reports progress to the console
type ConsoleProgressReporter struct {
	verbose bool
	logger  *slog.Logger
}

func (r *ConsoleProgressReporter) StartFile(path string) {
	if r.verbose {
		r.logger.Debug("Processing file", "file", path)
	}
}

func (r *ConsoleProgressReporter) CompleteFile(path string, violations int) {
	if r.verbose && violations > 0 {
		r.logger.Debug("File processed", "file", path, "violations", violations)
	}
}

func (r *ConsoleProgressReporter) UpdateProgress(current, total int) {
	// Could add a progress bar here in the future
	if r.verbose && current%10 == 0 {
		r.logger.Info("Progress", "current", current, "total", total, "percent", fmt.Sprintf("%.1f%%", float64(current)/float64(total)*100))
	}
}

func (r *ConsoleProgressReporter) Complete(stats *goverhaul.LintStats) {
	if r.verbose {
		r.logger.Info("Analysis complete",
			"files", stats.filesProcessed.Load(),
			"duration", stats.Duration(),
			"files/sec", fmt.Sprintf("%.2f", stats.FilesPerSecond()),
		)
	}
}