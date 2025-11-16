package goverhaul

import (
	"errors"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/mod/modfile"
)

// Error types for linting
var (
	// ErrLint is returned when linting errors are found
	ErrLint = errors.New("lint errors found")
)

type Goverhaul struct {
	cfg    Config
	logger *slog.Logger
	cache  *Cache

	fs afero.Fs
}

func NewLinter(cfg Config, logger *slog.Logger, fs afero.Fs) (*Goverhaul, error) {
	linter := &Goverhaul{
		fs:     fs,
		cfg:    cfg,
		logger: ensureLogger(logger),
	}

	// Load cache for incremental analysis if enabled
	if cfg.Incremental {
		cache, err := linter.initializeCache(cfg.CacheFile)
		if err != nil {
			return nil, err
		}
		linter.cache = cache
	}

	return linter, nil
}

// Lint analyzes Go files in the given path for import rule violations
func (g *Goverhaul) Lint(path string) (*LintViolations, error) {
	// Walk the file system and check each file
	violations, err := g.walkAndLint(path)
	if err != nil {
		return nil, handleWalkError(err, path)
	}

	return violations, nil
}

// ensureLogger creates a default logger if none is provided
func ensureLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}
	return logger
}

// initializeCache sets up the cache for incremental analysis
func (g *Goverhaul) initializeCache(cachePath string) (*Cache, error) {
	g.logger.Info("Using incremental analysis with MUS encoding", "cache_file", cachePath)

	cache, err := NewCache(cachePath, g.fs)
	if err != nil {
		return nil, NewCacheError("failed to load cache", err)
	}
	return cache, nil
}

// walkAndLint walks the file system and lints each Go file
func (g *Goverhaul) walkAndLint(path string) (*LintViolations, error) {
	violations := NewLintViolations()

	// Use afero.Walk instead of filepath.Walk
	err := afero.Walk(g.fs, path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return WithDetails(WithFile(NewFSError("error accessing path", err), path),
				"Check if the path exists and you have permission to access it")
		}

		if !isGoFileFs(info) {
			return nil
		}

		// Check if we can skip this file based on cache
		if g.cfg.Incremental {
			cachedViolations := g.hasCachedViolations(path)
			if len(cachedViolations) > 0 {
				for _, v := range cachedViolations {
					violations.Add(v)
				}
				return nil
			}
		}
		err = g.lintFile(path, violations)
		return err
	})
	if err != nil {
		return nil, err
	}
	return violations, nil
}

// isGoFileFs checks if the file is a Go source file using afero.Fs.FileInfo
func isGoFileFs(info os.FileInfo) bool {
	return !info.IsDir() && strings.HasSuffix(info.Name(), ".go")
}

// hasCachedViolations checks if a file can be skipped based on cache
func (g *Goverhaul) hasCachedViolations(path string) []LintViolation {
	var cachedViolations LintViolations
	cachedViolations, err := g.cache.HasEntry(path)
	if err != nil {
		if errors.Is(err, ErrEntryNotFound) || errors.Is(err, ErrReadingCachedViolations) {
			// just log and continue the linting. LintCache checking should not halt the main operation.
			g.logger.Warn("Error checking file change status", "path", path, "error", err)
		}
	}

	return cachedViolations.Violations
}

// lintFile lints a single Go file
func (g *Goverhaul) lintFile(goFilePath string, violations *LintViolations) error {
	g.logger.Debug("Analyzing file", "path", goFilePath)

	// Use getImportsWithPositions to capture source locations for IDE integration
	imports, err := g.getImportsWithPositions(goFilePath)
	if err != nil {
		g.logger.Error("Could not parse file", "path", goFilePath, "error", err)
		// Continue with other files even if one fails to parse
		return nil
	}

	g.logger.Debug("Imports found", "path", goFilePath, "count", len(imports))

	for _, rule := range g.cfg.Rules {
		g.logger.Debug("Checking rule", "path", goFilePath, "rule_path", rule.Path, "applies", ruleAppliesToPath(rule, goFilePath))
		if !ruleAppliesToPath(rule, goFilePath) {
			continue
		}

		// Join the directory of the file being linted with the modfile name
		modfilePath := JoinPaths(DirPath(goFilePath), g.cfg.Modfile)
		fileViolations := g.checkImportsWithPositions(goFilePath, imports, rule, modfilePath)
		for _, v := range fileViolations {
			violations.Add(v)
		}

		// Update cache if incremental analysis is enabled
		if g.cfg.Incremental {
			g.updateCache(goFilePath, fileViolations)
		}
	}

	return nil
}

// ruleAppliesToPath checks if a rule applies to a given file path
func ruleAppliesToPath(rule Rule, filePath string) bool {
	rulePath := NormalizePath(rule.Path)
	currentDir := DirPath(filePath)

	// Convert paths to absolute if needed
	if !IsAbsPath(rulePath) && !IsAbsPath(currentDir) {
		absPath := AbsPath(filePath)
		absDir := DirPath(absPath)

		// If the current directory is not absolute (relative to working dir)
		// and the rule path is also relative (to project root)
		// then we need to check if the absolute path ends with the rule path
		// or if it's a subdirectory of the rule path
		if strings.HasSuffix(absDir, rulePath) || IsSubPath(rulePath, absDir) {
			return true
		}
	}

	// Check if the current directory matches the rule path exactly or is a subdirectory
	return currentDir == rulePath || IsSubPath(rulePath, currentDir)
}

// updateCache updates the cache with file violations
func (g *Goverhaul) updateCache(path string, fileViolations []LintViolation) {
	var err error
	if len(fileViolations) == 0 {
		err = g.cache.AddFile(path)
	} else {
		err = g.cache.AddFileWithViolations(path, fileViolations)
	}
	if err != nil {
		g.logger.Warn("Failed to update cache for file", "path", path, "error", err)
	}
}

// handleWalkError handles errors that occur during file system walking
func handleWalkError(err error, path string) error {
	if os.IsPermission(err) {
		return WithDetails(NewFSError("permission denied while walking the path", err),
			"Path: "+path+". Check if you have the necessary permissions.")
	} else if os.IsNotExist(err) {
		return WithDetails(NewFSError("path does not exist", err),
			"Path: "+path+". Check if the path exists.")
	} else {
		return WithDetails(NewLintError("error walking the path", err),
			"Path: "+path)
	}
}

func getModuleName(fs afero.Fs, modfilePath string) (string, error) {
	// If modfilePath is empty, default to "go.mod" in the current directory
	if modfilePath == "" {
		modfilePath = "go.mod"
	}

	goModBytes, err := afero.ReadFile(fs, modfilePath)
	if err != nil {
		return "", WithDetails(WithFile(NewFSError("failed to read go.mod file", err), modfilePath),
			"Module name is required for import path resolution")
	}

	modulePath := modfile.ModulePath(goModBytes)
	if modulePath == "" {
		// The file exists but doesn't contain a valid module declaration
		return "", WithDetails(WithFile(NewParseError("failed to extract module path from go.mod", nil), modfilePath),
			"The go.mod file exists but doesn't contain a valid module declaration")
	}

	return modulePath, nil
}

// ImportSpec represents an import with its source location
// This is needed for IDE integration to highlight violations at the correct location
type ImportSpec struct {
	Path     string    // Import path (e.g., "fmt", "internal/pkg")
	Position *Position // Source location in file
}

// getImportsWithFs gets imports from a Go file using afero.Fs
// Deprecated: Use getImportsWithPositions for better IDE integration
func (g *Goverhaul) getImports(path string) ([]string, error) {
	fset := token.NewFileSet()

	// Read the file content using afero.Fs
	content, err := afero.ReadFile(g.fs, path)
	if err != nil {
		return nil, WithDetails(WithFile(NewFSError("failed to read Go file", err), path),
			"Make sure the file exists and is readable")
	}

	// Parse the file content
	file, err := parser.ParseFile(fset, path, content, parser.ImportsOnly)
	if err != nil {
		return nil, WithDetails(WithFile(NewParseError("failed to parse Go file", err), path),
			"Make sure the file is a valid Go source file")
	}

	var imports []string
	for _, s := range file.Imports {
		imports = append(imports, strings.Trim(s.Path.Value, `"`))
	}

	return imports, nil
}

// getImportsWithPositions extracts all imports from a file with their source positions
// This is needed for IDE integration to highlight violations at the correct location
func (g *Goverhaul) getImportsWithPositions(path string) ([]ImportSpec, error) {
	fset := token.NewFileSet()

	// Read the file content using afero.Fs
	content, err := afero.ReadFile(g.fs, path)
	if err != nil {
		return nil, WithDetails(WithFile(NewFSError("failed to read Go file", err), path),
			"Make sure the file exists and is readable")
	}

	// Parse the file content using ImportsOnly mode for efficiency
	file, err := parser.ParseFile(fset, path, content, parser.ImportsOnly)
	if err != nil {
		return nil, WithDetails(WithFile(NewParseError("failed to parse Go file", err), path),
			"Make sure the file is a valid Go source file")
	}

	var imports []ImportSpec
	for _, s := range file.Imports {
		// Get position of the import path string (not the entire import statement)
		pos := fset.Position(s.Path.Pos())
		end := fset.Position(s.Path.End())

		imports = append(imports, ImportSpec{
			Path: strings.Trim(s.Path.Value, `"`),
			Position: &Position{
				Line:      pos.Line,
				Column:    pos.Column,
				Offset:    pos.Offset,
				EndLine:   end.Line,
				EndColumn: end.Column,
			},
		})
	}

	return imports, nil
}

// checkImportsWithPositions checks all imports in a file against a rule with position tracking
// This version captures source positions for IDE integration
func (g *Goverhaul) checkImportsWithPositions(path string, imports []ImportSpec, rule Rule, moduleName string) []LintViolation {
	violations := make([]LintViolation, 0)

	g.logger.Debug("Checking imports with positions", "path", path, "rule_path", rule.Path, "modfile_path", moduleName)

	// Create a rule matcher with the provided file system
	matcher := newRuleMatcherWithFs(rule, moduleName, g.fs)

	g.logger.Debug("Rule matcher created", "path", path, "rule_path", rule.Path, "module_name", matcher.moduleName)
	g.logger.Debug("Prohibited imports", "path", path, "prohibited", matcher.prohibitedMap)

	// Normalize the path for consistent reporting
	normalizedPath := NormalizePath(path)

	// Check each import with position information
	for _, importSpec := range imports {
		g.logger.Debug("Checking import", "path", path, "import", importSpec.Path, "position", importSpec.Position)
		violation := matcher.CheckImport(importSpec.Path, normalizedPath, g.logger, importSpec.Position)
		if violation != nil {
			g.logger.Debug("Violation found", "path", path, "import", importSpec.Path, "rule", rule.Path, "position", importSpec.Position)
			violations = append(violations, *violation)
		}
	}

	return violations
}

// prohibitedInfo holds the cause and severity for a prohibited import
type prohibitedInfo struct {
	cause    string
	severity Severity
}

// RuleMatcher encapsulates the logic for matching imports against rules
type RuleMatcher struct {
	rule          Rule
	moduleName    string
	allowedSet    map[string]bool
	prohibitedMap map[string]prohibitedInfo
}

// newRuleMatcherWithFs creates a new RuleMatcher using a custom Fs
func newRuleMatcherWithFs(rule Rule, moduleNameOrPath string, fs afero.Fs) *RuleMatcher {
	// Extract module name if moduleNameOrPath is a path to go.mod
	moduleName := moduleNameOrPath

	// First check if the file exists at the given path
	if strings.HasSuffix(moduleNameOrPath, ".mod") {
		fileInfo, err := fs.Stat(moduleNameOrPath)
		if err == nil && !fileInfo.IsDir() {
			extractedName, err := getModuleName(fs, moduleNameOrPath)
			if err == nil {
				moduleName = extractedName
			}
		} else {
			// Try to find go.mod in the project root
			rootModPath := "go.mod"
			extractedName, err := getModuleName(fs, rootModPath)
			if err == nil {
				moduleName = extractedName
			}
		}
	}

	matcher := &RuleMatcher{
		rule:          rule,
		moduleName:    moduleName,
		allowedSet:    make(map[string]bool),
		prohibitedMap: make(map[string]prohibitedInfo),
	}

	// Prepare allowed set
	for _, allowed := range rule.Allowed {
		// Add the original path to the allowed set
		matcher.allowedSet[allowed] = true

		// Handle module-relative paths (only for simple paths without dots)
		if !strings.Contains(allowed, ".") {
			matcher.allowedSet[strings.Join([]string{moduleName, allowed}, "/")] = true
		}
	}

	// Prepare prohibited map
	for _, prohibited := range rule.Prohibited {
		info := prohibitedInfo{
			cause:    prohibited.Cause,
			severity: prohibited.GetSeverity(),
		}
		// Add the original path to the prohibited map
		matcher.prohibitedMap[prohibited.Name] = info

		// Handle module-relative paths (only for simple paths without dots)
		if !strings.Contains(prohibited.Name, ".") {
			matcher.prohibitedMap[strings.Join([]string{moduleName, prohibited.Name}, "/")] = info
		}
	}

	return matcher
}

// IsProhibited checks if an import is prohibited by the rule
// Returns the cause, severity, and whether the import is prohibited
func (m *RuleMatcher) IsProhibited(imp string) (string, Severity, bool) {
	// Direct lookup for exact match
	if info, exists := m.prohibitedMap[imp]; exists {
		return info.cause, info.severity, true
	}

	// Check if the import path contains any of the prohibited paths
	for prohibitedPath, info := range m.prohibitedMap {
		// Skip module-prefixed paths to avoid duplicates
		if strings.Contains(prohibitedPath, "/") && !strings.HasPrefix(prohibitedPath, m.moduleName) {
			if strings.HasSuffix(imp, prohibitedPath) {
				return info.cause, info.severity, true
			}
		}
	}

	return "", SeverityError, false
}

// IsAllowed checks if an import is allowed by the rule
func (m *RuleMatcher) IsAllowed(imp string) bool {
	// If there are no allowed imports specified, all imports are allowed
	if len(m.rule.Allowed) == 0 {
		return true
	}
	return m.allowedSet[imp]
}

// createViolation creates a LintViolation with the given parameters
func createViolation(file, imp, rule, cause, details string, severity Severity, position *Position) *LintViolation {
	return &LintViolation{
		File:     file,
		Import:   imp,
		Rule:     rule,
		Cause:    cause,
		Details:  details,
		Severity: severity,
		Position: position,
	}
}

// logAndCreateViolation logs an error and creates a violation
func (m *RuleMatcher) logAndCreateViolation(logger *slog.Logger, file, imp, message, cause, details string, severity Severity, position *Position) *LintViolation {
	// Log based on severity level
	logArgs := []any{"file", file, "import", imp, "severity", severity.String()}
	if cause != "" {
		logArgs = append(logArgs, "cause", cause)
	}

	switch severity {
	case SeverityError:
		logger.Error(message, logArgs...)
	case SeverityWarning:
		logger.Warn(message, logArgs...)
	case SeverityInfo, SeverityHint:
		logger.Info(message, logArgs...)
	}

	return createViolation(file, imp, m.rule.Path, cause, details, severity, position)
}

// CheckImport checks a single import against the rule
// Position parameter is optional - pass nil if not available (backward compatibility)
func (m *RuleMatcher) CheckImport(imp string, normalizedPath string, logger *slog.Logger, position *Position) *LintViolation {
	// First check if the import is prohibited
	cause, severity, isProhibited := m.IsProhibited(imp)
	if isProhibited {
		details := "This import is explicitly prohibited"
		if cause != "" {
			details += " with cause: " + cause
		}

		return m.logAndCreateViolation(logger, normalizedPath, imp, "Import is prohibited", cause, details, severity, position)
	}

	// Then check if the import is allowed
	if !m.IsAllowed(imp) {
		details := "This import is not in the allowed list for this package"
		// For "not allowed" violations, default to error severity
		return m.logAndCreateViolation(logger, normalizedPath, imp, "Import is not allowed", "", details, SeverityError, position)
	}

	return nil
}

// checkImports checks all imports in a file against a rule using the provided file system
func (g *Goverhaul) checkImports(path string, imports []string, rule Rule, moduleName string) []LintViolation {
	violations := make([]LintViolation, 0)

	g.logger.Debug("Checking imports", "path", path, "rule_path", rule.Path, "modfile_path", moduleName)

	// Create a rule matcher with the provided file system
	matcher := newRuleMatcherWithFs(rule, moduleName, g.fs)

	g.logger.Debug("Rule matcher created", "path", path, "rule_path", rule.Path, "module_name", matcher.moduleName)
	g.logger.Debug("Prohibited imports", "path", path, "prohibited", matcher.prohibitedMap)

	// Normalize the path for consistent reporting
	normalizedPath := NormalizePath(path)

	// Check each import (without position information)
	for _, imp := range imports {
		g.logger.Debug("Checking import", "path", path, "import", imp)
		violation := matcher.CheckImport(imp, normalizedPath, g.logger, nil)
		if violation != nil {
			g.logger.Debug("Violation found", "path", path, "import", imp, "rule", rule.Path)
			violations = append(violations, *violation)
		}
	}

	return violations
}
