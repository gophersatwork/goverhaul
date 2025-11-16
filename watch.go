package goverhaul

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/afero"
)

// WatchMode provides continuous file monitoring and re-analysis
type WatchMode struct {
	linter     *Goverhaul
	config     Config
	configPath string
	logger     *slog.Logger
	fs         afero.Fs

	watcher      *fsnotify.Watcher
	debounceTime time.Duration

	// Debouncing state
	mu             sync.Mutex
	pendingChanges map[string]time.Time
	debounceTimer  *time.Timer

	// Formatting options
	groupByRule bool
	colorMode   string

	// Statistics
	stats WatchStats
}

// WatchStats holds statistics about watch mode operation
type WatchStats struct {
	mu               sync.Mutex
	totalAnalyses    int
	filesAnalyzed    int
	violationsFound  int
	lastAnalysisTime time.Time
}

// WatchConfig holds configuration for watch mode
type WatchConfig struct {
	Path         string
	ConfigPath   string
	Logger       *slog.Logger
	FS           afero.Fs
	DebounceTime time.Duration
	GroupByRule  bool
	ColorMode    string
}

// NewWatchMode creates a new WatchMode instance
func NewWatchMode(cfg WatchConfig) (*WatchMode, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	if cfg.FS == nil {
		cfg.FS = afero.NewOsFs()
	}

	if cfg.DebounceTime == 0 {
		cfg.DebounceTime = 100 * time.Millisecond
	}

	// Load configuration
	config, err := LoadConfig(cfg.FS, cfg.Path, cfg.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create linter
	linter, err := NewLinter(config, cfg.Logger, cfg.FS)
	if err != nil {
		return nil, fmt.Errorf("failed to create linter: %w", err)
	}

	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	wm := &WatchMode{
		linter:         linter,
		config:         config,
		configPath:     cfg.ConfigPath,
		logger:         cfg.Logger,
		fs:             cfg.FS,
		watcher:        watcher,
		debounceTime:   cfg.DebounceTime,
		pendingChanges: make(map[string]time.Time),
		groupByRule:    cfg.GroupByRule,
		colorMode:      cfg.ColorMode,
	}

	return wm, nil
}

// Start begins watching for file changes
func (w *WatchMode) Start(ctx context.Context, path string) error {
	// Initial analysis
	w.printHeader()
	w.logger.Info("Starting watch mode", "path", path)

	if err := w.runInitialAnalysis(path); err != nil {
		return fmt.Errorf("initial analysis failed: %w", err)
	}

	// Add all Go files to watcher
	if err := w.addGoFilesToWatcher(path); err != nil {
		return fmt.Errorf("failed to add files to watcher: %w", err)
	}

	// Watch config file if specified
	if w.configPath != "" {
		if err := w.watchConfigFile(w.configPath); err != nil {
			w.logger.Warn("Failed to watch config file", "path", w.configPath, "error", err)
		}
	}

	w.printWatchingMessage(path)

	// Start event processing
	return w.processEvents(ctx, path)
}

// Stop gracefully stops the watcher
func (w *WatchMode) Stop() error {
	if w.watcher != nil {
		return w.watcher.Close()
	}
	return nil
}

// runInitialAnalysis performs the first analysis
func (w *WatchMode) runInitialAnalysis(path string) error {
	violations, err := w.linter.Lint(path)
	if err != nil {
		return err
	}

	w.printViolations(violations)
	w.updateStats(len(violations.Violations))
	return nil
}

// addGoFilesToWatcher recursively adds all Go files and directories to the watcher
func (w *WatchMode) addGoFilesToWatcher(root string) error {
	return afero.Walk(w.fs, root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			w.logger.Warn("Error walking path", "path", path, "error", err)
			return nil // Continue walking
		}

		// Watch directories to detect new files
		if info.IsDir() {
			// Skip hidden directories and vendor
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" {
				return filepath.SkipDir
			}

			if err := w.watcher.Add(path); err != nil {
				w.logger.Warn("Failed to watch directory", "path", path, "error", err)
			}
			return nil
		}

		// We watch directories, not individual files for efficiency
		return nil
	})
}

// watchConfigFile adds the config file to the watcher
func (w *WatchMode) watchConfigFile(configPath string) error {
	// Resolve to absolute path
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return err
	}

	// Watch the directory containing the config file
	configDir := filepath.Dir(absPath)
	return w.watcher.Add(configDir)
}

// processEvents handles file system events with debouncing
func (w *WatchMode) processEvents(ctx context.Context, rootPath string) error {
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Stopping watch mode")
			return nil

		case event, ok := <-w.watcher.Events:
			if !ok {
				return fmt.Errorf("watcher events channel closed")
			}
			w.handleEvent(event, rootPath)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return fmt.Errorf("watcher errors channel closed")
			}
			w.logger.Error("Watcher error", "error", err)
		}
	}
}

// handleEvent processes a single file system event
func (w *WatchMode) handleEvent(event fsnotify.Event, rootPath string) {
	// Filter events we care about
	if !w.shouldProcessEvent(event) {
		return
	}

	// Check if it's the config file
	if w.isConfigFile(event.Name) {
		w.handleConfigChange(rootPath)
		return
	}

	// Only process Go files
	if !strings.HasSuffix(event.Name, ".go") {
		return
	}

	// Add to pending changes for debouncing
	w.mu.Lock()
	w.pendingChanges[event.Name] = time.Now()

	// Reset debounce timer
	if w.debounceTimer != nil {
		w.debounceTimer.Stop()
	}

	w.debounceTimer = time.AfterFunc(w.debounceTime, func() {
		w.processPendingChanges(rootPath)
	})
	w.mu.Unlock()
}

// shouldProcessEvent filters events we care about
func (w *WatchMode) shouldProcessEvent(event fsnotify.Event) bool {
	// We care about writes, creates, and renames
	return event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename)
}

// isConfigFile checks if the event is for the config file
func (w *WatchMode) isConfigFile(path string) bool {
	if w.configPath == "" {
		return false
	}

	absConfigPath, _ := filepath.Abs(w.configPath)
	absEventPath, _ := filepath.Abs(path)

	return absConfigPath == absEventPath
}

// handleConfigChange reloads config and re-analyzes everything
func (w *WatchMode) handleConfigChange(rootPath string) {
	w.printTimestamp()
	fmt.Println(color.New(color.FgYellow, color.Bold).Sprint("📝 Config file changed"))
	fmt.Println(color.New(color.FgCyan).Sprint("⚡ Reloading configuration and re-analyzing all files..."))

	// Reload configuration
	newConfig, err := LoadConfig(w.fs, rootPath, w.configPath)
	if err != nil {
		w.printError(fmt.Sprintf("Failed to reload config: %v", err))
		return
	}

	// Create new linter with updated config
	newLinter, err := NewLinter(newConfig, w.logger, w.fs)
	if err != nil {
		w.printError(fmt.Sprintf("Failed to create linter with new config: %v", err))
		return
	}

	w.linter = newLinter
	w.config = newConfig

	// Re-analyze everything
	violations, err := w.linter.Lint(rootPath)
	if err != nil {
		w.printError(fmt.Sprintf("Analysis failed: %v", err))
		return
	}

	w.printViolations(violations)
	w.updateStats(len(violations.Violations))
}

// processPendingChanges analyzes all pending file changes
func (w *WatchMode) processPendingChanges(rootPath string) {
	w.mu.Lock()
	changes := make([]string, 0, len(w.pendingChanges))
	for path := range w.pendingChanges {
		changes = append(changes, path)
	}
	w.pendingChanges = make(map[string]time.Time)
	w.mu.Unlock()

	if len(changes) == 0 {
		return
	}

	w.printTimestamp()
	for _, path := range changes {
		fmt.Println(color.New(color.FgCyan).Sprintf("📝 %s changed", path))
	}

	fileText := "file"
	if len(changes) > 1 {
		fileText = "files"
	}
	fmt.Println(color.New(color.FgMagenta).Sprintf("⚡ Re-analyzing %d %s...", len(changes), fileText))

	// Perform incremental analysis on changed files only
	violations := w.analyzeFiles(changes)

	w.printViolations(violations)
	w.updateStats(len(violations.Violations))
}

// analyzeFiles performs incremental analysis on specific files
func (w *WatchMode) analyzeFiles(files []string) *LintViolations {
	violations := NewLintViolations()

	for _, file := range files {
		// Check if file exists and is a Go file
		info, err := w.fs.Stat(file)
		if err != nil {
			if os.IsNotExist(err) {
				w.logger.Debug("File was deleted, skipping", "path", file)
				continue
			}
			w.logger.Warn("Failed to stat file", "path", file, "error", err)
			continue
		}

		if info.IsDir() || !strings.HasSuffix(file, ".go") {
			continue
		}

		// Lint the file
		if err := w.linter.lintFile(file, violations); err != nil {
			w.logger.Error("Failed to lint file", "path", file, "error", err)
		}
	}

	return violations
}

// printHeader prints the initial header
func (w *WatchMode) printHeader() {
	boxColor := color.New(color.FgHiBlack)
	titleColor := color.New(color.Bold)

	boxTop := "╭─────────────────────────────────────────────────────╮"
	boxBottom := "╰─────────────────────────────────────────────────────╯"

	fmt.Println(boxColor.Sprint(boxTop))
	fmt.Println(boxColor.Sprint("│") + "  " + titleColor.Sprint("Goverhaul Watch Mode") + strings.Repeat(" ", 31) + boxColor.Sprint("│"))
	fmt.Println(boxColor.Sprint(boxBottom))
	fmt.Println()
}

// printWatchingMessage prints the watching message
func (w *WatchMode) printWatchingMessage(path string) {
	fmt.Println()
	watchMsg := fmt.Sprintf("👀 Watching %s for changes...", path)
	fmt.Println(color.New(color.FgGreen, color.Bold).Sprint(watchMsg))
	fmt.Println(color.New(color.FgHiBlack).Sprint("Press Ctrl+C to stop"))
	fmt.Println()
}

// printTimestamp prints the current timestamp
func (w *WatchMode) printTimestamp() {
	timestamp := time.Now().Format("15:04:05")
	fmt.Printf("[%s] ", color.New(color.FgHiBlack).Sprint(timestamp))
}

// printViolations formats and prints violations
func (w *WatchMode) printViolations(violations *LintViolations) {
	if violations.IsEmpty() {
		fmt.Println(color.New(color.FgGreen, color.Bold).Sprint("✅ No violations found"))
		fmt.Println()
		return
	}

	fmt.Println(color.New(color.FgRed, color.Bold).Sprintf("❌ Found %d violation(s)", len(violations.Violations)))
	fmt.Println()

	// Format violations - use simple grouped output
	if w.groupByRule {
		fmt.Print(w.formatViolationsByRule(violations))
	} else {
		fmt.Print(w.formatViolationsByFile(violations))
	}
}

// formatViolationsByFile formats violations grouped by file
func (w *WatchMode) formatViolationsByFile(violations *LintViolations) string {
	var output strings.Builder

	fileViolations := make(map[string][]LintViolation)
	for _, v := range violations.Violations {
		fileViolations[v.File] = append(fileViolations[v.File], v)
	}

	for file, viols := range fileViolations {
		output.WriteString(color.New(color.FgCyan, color.Bold).Sprintf("  📁 %s\n", file))
		output.WriteString(color.HiBlackString("     (%d violations)\n\n", len(viols)))

		for _, v := range viols {
			w.formatSingleViolation(&output, &v)
		}

		output.WriteString("\n")
	}

	return output.String()
}

// formatViolationsByRule formats violations grouped by rule
func (w *WatchMode) formatViolationsByRule(violations *LintViolations) string {
	var output strings.Builder

	ruleViolations := make(map[string][]LintViolation)
	for _, v := range violations.Violations {
		ruleViolations[v.Rule] = append(ruleViolations[v.Rule], v)
	}

	for rule, viols := range ruleViolations {
		output.WriteString(color.New(color.FgYellow, color.Bold).Sprintf("  📋 Rule: %s\n", rule))
		output.WriteString(color.HiBlackString("     (%d violations)\n\n", len(viols)))

		for _, v := range viols {
			w.formatSingleViolation(&output, &v)
		}

		output.WriteString("\n")
	}

	return output.String()
}

// formatSingleViolation formats a single violation
func (w *WatchMode) formatSingleViolation(output *strings.Builder, v *LintViolation) {
	severity := v.Severity
	if severity == "" {
		severity = SeverityError
	}

	var icon string
	var severityColor *color.Color

	switch severity {
	case SeverityError:
		icon = "❌"
		severityColor = color.New(color.FgRed, color.Bold)
	case SeverityWarning:
		icon = "⚠️ "
		severityColor = color.New(color.FgYellow, color.Bold)
	case SeverityInfo:
		icon = "ℹ️ "
		severityColor = color.New(color.FgBlue, color.Bold)
	case SeverityHint:
		icon = "💡"
		severityColor = color.New(color.FgHiBlack, color.Bold)
	default:
		icon = "❌"
		severityColor = color.New(color.FgRed, color.Bold)
	}

	position := ""
	if v.Position != nil && v.Position.IsValid() {
		position = fmt.Sprintf(" · line %d:%d", v.Position.Line, v.Position.Column)
	}

	output.WriteString("     ")
	output.WriteString(icon)
	output.WriteString(" import ")
	output.WriteString(color.New(color.FgMagenta).Sprintf("\"%s\"", v.Import))
	output.WriteString(position)
	output.WriteString("\n")

	output.WriteString("        ")
	output.WriteString(color.HiBlackString("Rule: "))
	output.WriteString(color.New(color.FgYellow).Sprint(v.Rule))
	output.WriteString("\n")

	output.WriteString("        ")
	output.WriteString(color.HiBlackString("Severity: "))
	output.WriteString(severityColor.Sprint(string(severity)))
	output.WriteString("\n")

	if v.Cause != "" {
		output.WriteString("        ")
		output.WriteString(v.Cause)
		output.WriteString("\n")
	}

	if v.Details != "" {
		output.WriteString("        ")
		output.WriteString(color.HiBlackString("Details: "))
		output.WriteString(v.Details)
		output.WriteString("\n")
	}

	output.WriteString("\n")
}

// printError prints an error message
func (w *WatchMode) printError(msg string) {
	fmt.Println(color.New(color.FgRed, color.Bold).Sprint("❌ Error: ") + msg)
	fmt.Println()
}

// updateStats updates watch mode statistics
func (w *WatchMode) updateStats(violations int) {
	w.stats.mu.Lock()
	defer w.stats.mu.Unlock()

	w.stats.totalAnalyses++
	w.stats.violationsFound += violations
	w.stats.lastAnalysisTime = time.Now()
}

// GetStats returns current watch mode statistics
func (w *WatchMode) GetStats() WatchStats {
	w.stats.mu.Lock()
	defer w.stats.mu.Unlock()
	return w.stats
}
