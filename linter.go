package goverhaul

import (
	"errors"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"strings"

	"github.com/gophersatwork/granular"
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
	cache  *LintCache

	fs afero.Fs
}

func NewLinter(cfg Config, logger *slog.Logger, fs afero.Fs) (*Goverhaul, error) {
	linter := &Goverhaul{
		fs:     fs,
		cfg:    cfg,
		logger: ensureLogger(logger),
	}

	// Load cache for incremental analysis if enabled
	var cache LintCache
	var err error
	if cfg.Incremental {
		cache, err = linter.initializeCache(cfg.CacheFile)
		linter.cache = &cache
		if err != nil {
			return nil, err
		}
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
func (g *Goverhaul) initializeCache(cachePath string) (LintCache, error) {
	g.logger.Info("Using incremental analysis", "cache_file", cachePath)

	gCache, err := granular.New(cachePath, granular.WithFs(g.fs))
	if err != nil {
		return LintCache{}, NewCacheError("failed to load cache", err)
	}
	return LintCache{gCache: gCache}, nil
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
	imports, err := g.getImports(goFilePath)
	if err != nil {
		g.logger.Error("Could not parse file", "path", goFilePath, "error", err)
		// Continue with other files even if one fails to parse
		return nil
	}

	g.logger.Debug("Imports found", "path", goFilePath, "imports", imports)

	for _, rule := range g.cfg.Rules {
		g.logger.Debug("Checking rule", "path", goFilePath, "rule_path", rule.Path, "applies", ruleAppliesToPath(rule, goFilePath))
		if !ruleAppliesToPath(rule, goFilePath) {
			continue
		}

		// Join the directory of the file being linted with the modfile name
		modfilePath := JoinPaths(DirPath(goFilePath), g.cfg.Modfile)
		fileViolations := g.checkImports(goFilePath, imports, rule, modfilePath)
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

// getImportsWithFs gets imports from a Go file using afero.Fs
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

// RuleMatcher encapsulates the logic for matching imports against rules
type RuleMatcher struct {
	rule          Rule
	moduleName    string
	allowedSet    map[string]bool
	prohibitedMap map[string]string
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
		prohibitedMap: make(map[string]string),
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
		// Add the original path to the prohibited map
		matcher.prohibitedMap[prohibited.Name] = prohibited.Cause

		// Handle module-relative paths (only for simple paths without dots)
		if !strings.Contains(prohibited.Name, ".") {
			matcher.prohibitedMap[strings.Join([]string{moduleName, prohibited.Name}, "/")] = prohibited.Cause
		}
	}

	return matcher
}

// IsProhibited checks if an import is prohibited by the rule
func (m *RuleMatcher) IsProhibited(imp string) (string, bool) {
	// Direct lookup for exact match
	if cause, exists := m.prohibitedMap[imp]; exists {
		return cause, true
	}

	// Check if the import path contains any of the prohibited paths
	for prohibitedPath, cause := range m.prohibitedMap {
		// Skip module-prefixed paths to avoid duplicates
		if strings.Contains(prohibitedPath, "/") && !strings.HasPrefix(prohibitedPath, m.moduleName) {
			if strings.HasSuffix(imp, prohibitedPath) {
				return cause, true
			}
		}
	}

	return "", false
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
func createViolation(file, imp, rule, cause, details string) *LintViolation {
	return &LintViolation{
		File:    file,
		Import:  imp,
		Rule:    rule,
		Cause:   cause,
		Details: details,
	}
}

// logAndCreateViolation logs an error and creates a violation
func (m *RuleMatcher) logAndCreateViolation(logger *slog.Logger, file, imp, message, cause, details string) *LintViolation {
	if cause != "" {
		logger.Error(message,
			"file", file,
			"import", imp,
			"cause", cause)
	} else {
		logger.Error(message,
			"file", file,
			"import", imp)
	}

	return createViolation(file, imp, m.rule.Path, cause, details)
}

// CheckImport checks a single import against the rule
func (m *RuleMatcher) CheckImport(imp string, normalizedPath string, logger *slog.Logger) *LintViolation {
	// First check if the import is prohibited
	cause, isProhibited := m.IsProhibited(imp)
	if isProhibited {
		details := "This import is explicitly prohibited"
		if cause != "" {
			details += " with cause: " + cause
		}

		return m.logAndCreateViolation(logger, normalizedPath, imp, "Import is prohibited", cause, details)
	}

	// Then check if the import is allowed
	if !m.IsAllowed(imp) {
		details := "This import is not in the allowed list for this package"
		return m.logAndCreateViolation(logger, normalizedPath, imp, "Import is not allowed", "", details)
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

	// Check each import
	for _, imp := range imports {
		g.logger.Debug("Checking import", "path", path, "import", imp)
		violation := matcher.CheckImport(imp, normalizedPath, g.logger)
		if violation != nil {
			g.logger.Debug("Violation found", "path", path, "import", imp, "rule", rule.Path)
			violations = append(violations, *violation)
		}
	}

	return violations
}
