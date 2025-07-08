package main

import (
	"errors"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gophersatwork/granular"
	"golang.org/x/mod/modfile"
)

// Error types for linting
var (
	// ErrLint is returned when linting errors are found
	ErrLint = errors.New("lint errors found")
)

func Lint(path string, cfg Config, logger *slog.Logger) error {
	// Use the provided logger or create a default one
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	// Load cache for incremental analysis if enabled
	var cache LintCache
	if cfg.Incremental {
		cachePath := cfg.CacheFile
		logger.Info("Using incremental analysis", "cache_file", cachePath)

		// granular cache interactions
		gCache, err := granular.New(cachePath)
		if err != nil {
			return NewFSError("failed to load cache", err)
		}
		cache = LintCache{gCache: gCache}
	}

	// Create a violations collection to track all linting errors
	violations := NewLintViolations()

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return NewFSError("error accessing path", err).
				WithFile(path).
				WithDetails("Check if the path exists and you have permission to access it")
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".go") {
			// Skip unchanged files if incremental analysis is enabled
			if cfg.Incremental {
				cachedViolations, err := cache.hasEntry(path)
				if err != nil {
					if errors.Is(err, ErrEntryNotFound) || errors.Is(err, ErrReadingCachedViolations) {
						// just log and continue the linting. LintCache checking should not halt the main operation.
						logger.Warn("Error checking file change status", "path", path, "error", err)
					}
				} else {
					// File found in cache! Does it have previous violations?
					// If it is the case, let's put it in the current violations list.
					if !cachedViolations.IsEmpty() {
						for _, v := range cachedViolations.Violations {
							violations.Add(v)
						}
					}
					logger.Debug("Skipping unchanged file", "path", path)
					return nil
				}
			}

			logger.Debug("Analyzing file", "path", path)
			imports, err := getImports(path)
			if err != nil {
				logger.Error("Could not parse file", "path", path, "error", err)
				// Continue with other files even if one fails to parse
				return nil
			}

			for _, rule := range cfg.Rules {
				rulePath := filepath.ToSlash(rule.Path)
				currentDir := filepath.ToSlash(filepath.Dir(path))

				// Convert paths to absolute if needed
				if !filepath.IsAbs(rulePath) && !filepath.IsAbs(currentDir) {
					absPath, err := filepath.Abs(path)
					if err == nil {
						absDir := filepath.ToSlash(filepath.Dir(absPath))

						// If the current directory is not absolute (relative to working dir)
						// and the rule path is also relative (to project root)
						// then we need to check if the absolute path ends with the rule path
						// or if it's a subdirectory of the rule path
						if strings.HasSuffix(absDir, rulePath) || strings.Contains(absDir+"/", "/"+rulePath+"/") {
							fileViolations := checkImports(path, imports, rule, cfg.Modfile, logger)
							for _, v := range fileViolations {
								violations.Add(v)
							}
							// Update cache
							if cfg.Incremental {
								if len(fileViolations) == 0 {
									err = cache.AddFile(path)
								} else {
									err = cache.AddFileWithViolations(path, fileViolations)
								}
								if err != nil {
									logger.Warn("Failed to update cache for file", "path", path, "error", err)
								}
							}
							continue
						}
					}
				}

				// Check if the current directory matches the rule path exactly or is a subdirectory
				if currentDir == rulePath || strings.HasPrefix(currentDir, rulePath+"/") {
					fileViolations := checkImports(path, imports, rule, cfg.Modfile, logger)
					for _, v := range fileViolations {
						violations.Add(v)
					}
					// Update cache
					if cfg.Incremental {
						if len(fileViolations) == 0 {
							err = cache.AddFile(path)
						} else {
							err = cache.AddFileWithViolations(path, fileViolations)
						}
						if err != nil {
							logger.Warn("Failed to update cache for file", "path", path, "error", err)
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		// Check if the error is a file access issue
		if os.IsPermission(err) {
			return NewFSError("permission denied while walking the path", err).
				WithDetails("Path: " + path + ". Check if you have the necessary permissions.")
		} else if os.IsNotExist(err) {
			return NewFSError("path does not exist", err).
				WithDetails("Path: " + path + ". Check if the path exists.")
		} else {
			return NewLintError("error walking the path", err).
				WithDetails("Path: " + path)
		}
	}

	if !violations.IsEmpty() {
		// Log a summary of violations
		logger.Error("Found rule violations", "count", len(violations.Violations))
		return violations
	}

	logger.Info("No rule violations found. All dependencies are compliant!")
	return nil
}

func getModuleName(modfilePath string) (string, error) {
	goModBytes, err := os.ReadFile(modfilePath)
	if err != nil {
		return "", NewFSError("failed to read go.mod file", err).
			WithFile(modfilePath).
			WithDetails("Module name is required for import path resolution")
	}

	modulePath := modfile.ModulePath(goModBytes)
	if modulePath == "" {
		return "", NewParseError("failed to extract module path from go.mod", nil).
			WithFile(modfilePath).
			WithDetails("The go.mod file may be malformed or empty")
	}

	return modulePath, nil
}

func getImports(path string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, NewParseError("failed to parse Go file", err).
			WithFile(path).
			WithDetails("Make sure the file is a valid Go source file")
	}

	var imports []string
	for _, s := range file.Imports {
		imports = append(imports, strings.Trim(s.Path.Value, `"`))
	}

	return imports, nil
}

// prepareAllowedSet creates a map of allowed imports for a rule
func prepareAllowedSet(rule Rule, moduleName string) map[string]bool {
	allowedSet := make(map[string]bool)
	if len(rule.Allowed) > 0 {
		for _, allowed := range rule.Allowed {
			if !strings.Contains(allowed, ".") {
				allowedSet[allowed] = true
				allowedSet[strings.Join([]string{moduleName, allowed}, "/")] = true
			} else {
				allowedSet[allowed] = true
			}
		}
	}
	return allowedSet
}

// prepareProhibitedMap creates a map of prohibited imports for a rule
func prepareProhibitedMap(rule Rule, moduleName string) map[string]string {
	prohibitedMap := make(map[string]string) // Map from package name to cause
	for _, prohibited := range rule.Prohibited {
		if !strings.Contains(prohibited.Name, ".") {
			prohibitedMap[strings.Join([]string{moduleName, prohibited.Name}, "/")] = prohibited.Cause
		} else {
			prohibitedMap[prohibited.Name] = prohibited.Cause
		}
	}
	return prohibitedMap
}

func checkImports(path string, imports []string, rule Rule, moduleName string, logger *slog.Logger) []LintViolation {
	violations := make([]LintViolation, 0)

	// Prepare maps for allowed and prohibited imports
	allowedSet := prepareAllowedSet(rule, moduleName)
	prohibitedMap := prepareProhibitedMap(rule, moduleName)

	for _, imp := range imports {
		cause, isProhibited := prohibitedMap[imp]
		if isProhibited {
			violation := LintViolation{
				File:   filepath.ToSlash(path),
				Import: imp,
				Rule:   rule.Path,
				Cause:  cause,
			}

			if cause != "" {
				logger.Error("Import is prohibited",
					"file", filepath.ToSlash(path),
					"import", imp,
					"cause", cause)
				violation.Details = "This import is explicitly prohibited with cause: " + cause
			} else {
				logger.Error("Import is prohibited",
					"file", filepath.ToSlash(path),
					"import", imp)
				violation.Details = "This import is explicitly prohibited"
			}

			violations = append(violations, violation)
			continue // Skip allowed check if already prohibited
		}

		if len(rule.Allowed) > 0 && !allowedSet[imp] {
			logger.Error("Import is not allowed",
				"file", filepath.ToSlash(path),
				"import", imp)

			violation := LintViolation{
				File:    filepath.ToSlash(path),
				Import:  imp,
				Rule:    rule.Path,
				Details: "This import is not in the allowed list for this package",
			}
			violations = append(violations, violation)
		}
	}

	return violations
}