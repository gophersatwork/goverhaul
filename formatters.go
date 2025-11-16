package goverhaul

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// OutputFormat represents the output format type
type OutputFormat string

const (
	// FormatText outputs human-readable text (default)
	FormatText OutputFormat = "text"
	// FormatJSON outputs machine-readable JSON
	FormatJSON OutputFormat = "json"
	// FormatSARIF outputs SARIF 2.1.0 format for CI/CD integration
	FormatSARIF OutputFormat = "sarif"
	// FormatCheckstyle outputs Checkstyle XML format
	FormatCheckstyle OutputFormat = "checkstyle"
	// FormatJUnit outputs JUnit XML format
	FormatJUnit OutputFormat = "junit"
	// FormatMarkdown outputs Markdown format for documentation
	FormatMarkdown OutputFormat = "markdown"
)

// Formatter interface for different output formats
type Formatter interface {
	Format(violations *LintViolations, cfg *Config) ([]byte, error)
	ContentType() string
}

// FormatterFactory creates formatters based on the output format
func NewFormatter(format OutputFormat) (Formatter, error) {
	switch format {
	case FormatJSON:
		return &JSONFormatter{}, nil
	case FormatSARIF:
		return &SARIFFormatter{}, nil
	case FormatCheckstyle:
		return &CheckstyleFormatter{}, nil
	case FormatJUnit:
		return &JUnitFormatter{}, nil
	case FormatMarkdown:
		return &MarkdownFormatter{}, nil
	case FormatText:
		return &TextFormatter{}, nil
	default:
		return nil, fmt.Errorf("unsupported output format: %s", format)
	}
}

// JSONFormatter outputs violations in JSON format
type JSONFormatter struct {
	Pretty bool
}

// JSONOutput represents the JSON output structure
type JSONOutput struct {
	Summary    Summary          `json:"summary"`
	Violations []JSONViolation  `json:"violations"`
	Rules      []RuleSummary    `json:"rules"`
	Timestamp  string           `json:"timestamp"`
}

type Summary struct {
	TotalViolations int    `json:"total_violations"`
	FilesAnalyzed   int    `json:"files_analyzed"`
	FilesWithIssues int    `json:"files_with_issues"`
	Status          string `json:"status"`
}

type JSONViolation struct {
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	Rule     string `json:"rule"`
	Import   string `json:"import"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Cause    string `json:"cause,omitempty"`
}

type RuleSummary struct {
	Name       string `json:"name"`
	Violations int    `json:"violations"`
}

func (f *JSONFormatter) Format(violations *LintViolations, cfg *Config) ([]byte, error) {
	output := f.buildJSONOutput(violations, cfg)

	if f.Pretty {
		return json.MarshalIndent(output, "", "  ")
	}
	return json.Marshal(output)
}

func (f *JSONFormatter) ContentType() string {
	return "application/json"
}

func (f *JSONFormatter) buildJSONOutput(violations *LintViolations, cfg *Config) JSONOutput {
	// Count unique files
	fileMap := make(map[string]bool)
	ruleCount := make(map[string]int)

	jsonViolations := make([]JSONViolation, 0, len(violations.Violations))
	for _, v := range violations.Violations {
		fileMap[v.File] = true
		ruleCount[v.Rule]++

		jsonViolations = append(jsonViolations, JSONViolation{
			File:     v.File,
			Rule:     v.Rule,
			Import:   v.Import,
			Severity: "error", // Could be configurable
			Message:  fmt.Sprintf("Import '%s' violates rule '%s'", v.Import, v.Rule),
			Cause:    v.Cause,
		})
	}

	// Build rule summaries
	ruleSummaries := make([]RuleSummary, 0, len(ruleCount))
	for rule, count := range ruleCount {
		ruleSummaries = append(ruleSummaries, RuleSummary{
			Name:       rule,
			Violations: count,
		})
	}

	status := "passed"
	if len(violations.Violations) > 0 {
		status = "failed"
	}

	return JSONOutput{
		Summary: Summary{
			TotalViolations: len(violations.Violations),
			FilesAnalyzed:   len(fileMap),
			FilesWithIssues: len(fileMap),
			Status:          status,
		},
		Violations: jsonViolations,
		Rules:      ruleSummaries,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
}

// SARIFFormatter outputs violations in SARIF 2.1.0 format
type SARIFFormatter struct{}

// SARIF structures according to the SARIF 2.1.0 specification
type SARIFOutput struct {
	Schema  string    `json:"$schema"`
	Version string    `json:"version"`
	Runs    []SARIFRun `json:"runs"`
}

type SARIFRun struct {
	Tool    SARIFTool     `json:"tool"`
	Results []SARIFResult `json:"results"`
}

type SARIFTool struct {
	Driver SARIFDriver `json:"driver"`
}

type SARIFDriver struct {
	Name            string      `json:"name"`
	InformationURI  string      `json:"informationUri"`
	Version         string      `json:"version"`
	SemanticVersion string      `json:"semanticVersion"`
	Rules           []SARIFRule `json:"rules"`
}

type SARIFRule struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	ShortDescription SARIFMessage          `json:"shortDescription"`
	FullDescription  SARIFMessage          `json:"fullDescription"`
	Help             SARIFMessage          `json:"help"`
	DefaultConfig    SARIFRuleConfig       `json:"defaultConfiguration"`
	Properties       map[string]interface{} `json:"properties,omitempty"`
}

type SARIFMessage struct {
	Text string `json:"text"`
}

type SARIFRuleConfig struct {
	Level string `json:"level"`
}

type SARIFResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   SARIFMessage    `json:"message"`
	Locations []SARIFLocation `json:"locations"`
}

type SARIFLocation struct {
	PhysicalLocation SARIFPhysicalLocation `json:"physicalLocation"`
}

type SARIFPhysicalLocation struct {
	ArtifactLocation SARIFArtifactLocation `json:"artifactLocation"`
	Region           *SARIFRegion          `json:"region,omitempty"`
}

type SARIFArtifactLocation struct {
	URI string `json:"uri"`
}

type SARIFRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
	EndLine     int `json:"endLine"`
	EndColumn   int `json:"endColumn"`
}

func (f *SARIFFormatter) Format(violations *LintViolations, cfg *Config) ([]byte, error) {
	sarif := f.buildSARIFOutput(violations, cfg)
	return json.MarshalIndent(sarif, "", "  ")
}

func (f *SARIFFormatter) ContentType() string {
	return "application/sarif+json"
}

func (f *SARIFFormatter) buildSARIFOutput(violations *LintViolations, cfg *Config) SARIFOutput {
	// Build unique rules from violations
	ruleMap := make(map[string]*SARIFRule)
	results := make([]SARIFResult, 0, len(violations.Violations))

	for _, v := range violations.Violations {
		// Create or update rule
		if _, exists := ruleMap[v.Rule]; !exists {
			ruleMap[v.Rule] = &SARIFRule{
				ID:   v.Rule,
				Name: v.Rule,
				ShortDescription: SARIFMessage{
					Text: fmt.Sprintf("Architecture rule: %s", v.Rule),
				},
				FullDescription: SARIFMessage{
					Text: fmt.Sprintf("Enforces architectural boundaries for %s", v.Rule),
				},
				Help: SARIFMessage{
					Text: "Ensure imports comply with the defined architectural rules",
				},
				DefaultConfig: SARIFRuleConfig{
					Level: "error",
				},
			}
		}

		// Create result
		message := fmt.Sprintf("Import '%s' violates rule '%s'", v.Import, v.Rule)
		if v.Cause != "" {
			message = fmt.Sprintf("%s: %s", message, v.Cause)
		}

		results = append(results, SARIFResult{
			RuleID: v.Rule,
			Level:  "error",
			Message: SARIFMessage{
				Text: message,
			},
			Locations: []SARIFLocation{
				{
					PhysicalLocation: SARIFPhysicalLocation{
						ArtifactLocation: SARIFArtifactLocation{
							URI: v.File,
						},
						// Region would need line/column info from AST
					},
				},
			},
		})
	}

	// Convert rule map to slice
	rules := make([]SARIFRule, 0, len(ruleMap))
	for _, rule := range ruleMap {
		rules = append(rules, *rule)
	}

	return SARIFOutput{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []SARIFRun{
			{
				Tool: SARIFTool{
					Driver: SARIFDriver{
						Name:            "goverhaul",
						InformationURI:  "https://github.com/gophersatwork/goverhaul",
						Version:         "1.0.0", // Should be dynamic
						SemanticVersion: "1.0.0",
						Rules:           rules,
					},
				},
				Results: results,
			},
		},
	}
}

// CheckstyleFormatter outputs violations in Checkstyle XML format
type CheckstyleFormatter struct{}

type CheckstyleOutput struct {
	XMLName xml.Name          `xml:"checkstyle"`
	Version string            `xml:"version,attr"`
	Files   []CheckstyleFile  `xml:"file"`
}

type CheckstyleFile struct {
	Name   string            `xml:"name,attr"`
	Errors []CheckstyleError `xml:"error"`
}

type CheckstyleError struct {
	Line     int    `xml:"line,attr"`
	Column   int    `xml:"column,attr,omitempty"`
	Severity string `xml:"severity,attr"`
	Message  string `xml:"message,attr"`
	Source   string `xml:"source,attr"`
}

func (f *CheckstyleFormatter) Format(violations *LintViolations, cfg *Config) ([]byte, error) {
	// Group violations by file
	fileMap := make(map[string][]LintViolation)
	for _, v := range violations.Violations {
		fileMap[v.File] = append(fileMap[v.File], v)
	}

	files := make([]CheckstyleFile, 0, len(fileMap))
	for file, fileViolations := range fileMap {
		errors := make([]CheckstyleError, 0, len(fileViolations))
		for _, v := range fileViolations {
			message := fmt.Sprintf("Import '%s' violates rule '%s'", v.Import, v.Rule)
			if v.Cause != "" {
				message = fmt.Sprintf("%s: %s", message, v.Cause)
			}

			errors = append(errors, CheckstyleError{
				Line:     1, // Would need actual line numbers from AST
				Severity: "error",
				Message:  message,
				Source:   fmt.Sprintf("goverhaul.%s", v.Rule),
			})
		}

		files = append(files, CheckstyleFile{
			Name:   file,
			Errors: errors,
		})
	}

	output := CheckstyleOutput{
		Version: "8.0",
		Files:   files,
	}

	return xml.MarshalIndent(output, "", "  ")
}

func (f *CheckstyleFormatter) ContentType() string {
	return "application/xml"
}

// JUnitFormatter outputs violations in JUnit XML format
type JUnitFormatter struct{}

type JUnitTestSuites struct {
	XMLName    xml.Name         `xml:"testsuites"`
	Name       string           `xml:"name,attr"`
	Tests      int              `xml:"tests,attr"`
	Failures   int              `xml:"failures,attr"`
	Errors     int              `xml:"errors,attr"`
	Time       float64          `xml:"time,attr"`
	TestSuites []JUnitTestSuite `xml:"testsuite"`
}

type JUnitTestSuite struct {
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Errors    int             `xml:"errors,attr"`
	Time      float64         `xml:"time,attr"`
	TestCases []JUnitTestCase `xml:"testcase"`
}

type JUnitTestCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      float64       `xml:"time,attr"`
	Failure   *JUnitFailure `xml:"failure,omitempty"`
}

type JUnitFailure struct {
	Type    string `xml:"type,attr"`
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

func (f *JUnitFormatter) Format(violations *LintViolations, cfg *Config) ([]byte, error) {
	// Group violations by file
	fileMap := make(map[string][]LintViolation)
	for _, v := range violations.Violations {
		fileMap[v.File] = append(fileMap[v.File], v)
	}

	testSuites := make([]JUnitTestSuite, 0, len(fileMap))
	totalFailures := 0

	for file, fileViolations := range fileMap {
		testCases := make([]JUnitTestCase, 0, len(fileViolations))
		failures := 0

		for _, v := range fileViolations {
			message := fmt.Sprintf("Import '%s' violates rule '%s'", v.Import, v.Rule)
			if v.Cause != "" {
				message = fmt.Sprintf("%s: %s", message, v.Cause)
			}

			testCase := JUnitTestCase{
				Name:      fmt.Sprintf("Architecture rule: %s", v.Rule),
				ClassName: filepath.Base(file),
				Time:      0.001,
				Failure: &JUnitFailure{
					Type:    "ArchitectureViolation",
					Message: message,
					Text:    fmt.Sprintf("File: %s\nImport: %s\nRule: %s\nCause: %s", v.File, v.Import, v.Rule, v.Cause),
				},
			}
			testCases = append(testCases, testCase)
			failures++
		}

		testSuites = append(testSuites, JUnitTestSuite{
			Name:      file,
			Tests:     len(fileViolations),
			Failures:  failures,
			Errors:    0,
			Time:      0.001 * float64(len(fileViolations)),
			TestCases: testCases,
		})
		totalFailures += failures
	}

	output := JUnitTestSuites{
		Name:       "Goverhaul Architecture Tests",
		Tests:      len(violations.Violations),
		Failures:   totalFailures,
		Errors:     0,
		Time:       0.001 * float64(len(violations.Violations)),
		TestSuites: testSuites,
	}

	return xml.MarshalIndent(output, "", "  ")
}

func (f *JUnitFormatter) ContentType() string {
	return "application/xml"
}

// MarkdownFormatter outputs violations in Markdown format
type MarkdownFormatter struct{}

func (f *MarkdownFormatter) Format(violations *LintViolations, cfg *Config) ([]byte, error) {
	var sb strings.Builder

	sb.WriteString("# Goverhaul Architecture Violations Report\n\n")
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n\n", time.Now().UTC().Format(time.RFC3339)))

	if len(violations.Violations) == 0 {
		sb.WriteString("✅ **No violations found!**\n\n")
		sb.WriteString("Your codebase complies with all architectural rules.\n")
		return []byte(sb.String()), nil
	}

	// Summary
	fileMap := make(map[string][]LintViolation)
	ruleMap := make(map[string][]LintViolation)
	for _, v := range violations.Violations {
		fileMap[v.File] = append(fileMap[v.File], v)
		ruleMap[v.Rule] = append(ruleMap[v.Rule], v)
	}

	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Violations:** %d\n", len(violations.Violations)))
	sb.WriteString(fmt.Sprintf("- **Files with Issues:** %d\n", len(fileMap)))
	sb.WriteString(fmt.Sprintf("- **Rules Violated:** %d\n\n", len(ruleMap)))

	// Violations by Rule
	sb.WriteString("## Violations by Rule\n\n")
	for rule, ruleViolations := range ruleMap {
		sb.WriteString(fmt.Sprintf("### Rule: `%s` (%d violations)\n\n", rule, len(ruleViolations)))
		sb.WriteString("| File | Import | Cause |\n")
		sb.WriteString("|------|--------|-------|\n")

		for _, v := range ruleViolations {
			cause := v.Cause
			if cause == "" {
				cause = "-"
			}
			sb.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n", v.File, v.Import, cause))
		}
		sb.WriteString("\n")
	}

	// Violations by File
	sb.WriteString("## Violations by File\n\n")
	for file, fileViolations := range fileMap {
		sb.WriteString(fmt.Sprintf("### `%s` (%d violations)\n\n", file, len(fileViolations)))

		for _, v := range fileViolations {
			sb.WriteString(fmt.Sprintf("- **Rule:** `%s`\n", v.Rule))
			sb.WriteString(fmt.Sprintf("  - **Import:** `%s`\n", v.Import))
			if v.Cause != "" {
				sb.WriteString(fmt.Sprintf("  - **Cause:** %s\n", v.Cause))
			}
		}
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString("---\n\n")
	sb.WriteString("*Generated by [Goverhaul](https://github.com/gophersatwork/goverhaul) - Architectural Rules Enforcement for Go*\n")

	return []byte(sb.String()), nil
}

func (f *MarkdownFormatter) ContentType() string {
	return "text/markdown"
}

// TextFormatter outputs violations in human-readable text format
type TextFormatter struct{}

func (f *TextFormatter) Format(violations *LintViolations, cfg *Config) ([]byte, error) {
	// Use existing print methods
	return []byte(violations.PrintByFile()), nil
}

func (f *TextFormatter) ContentType() string {
	return "text/plain"
}