package goverhaul

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestViolations() *LintViolations {
	return &LintViolations{
		Violations: []LintViolation{
			{
				File:   "internal/api/handler.go",
				Import: "internal/database",
				Rule:   "api-no-db",
				Cause:  "API layer should not directly import database",
			},
			{
				File:   "internal/api/handler.go",
				Import: "unsafe",
				Rule:   "no-unsafe",
				Cause:  "Unsafe package is not allowed",
			},
			{
				File:   "internal/service/user.go",
				Import: "internal/api",
				Rule:   "service-no-api",
				Cause:  "Service layer should not import API layer",
			},
		},
	}
}

func createTestConfig() *Config {
	return &Config{
		Modfile: "go.mod",
		Rules: []Rule{
			{
				Path: "internal/api",
				Prohibited: []ProhibitedPkg{
					{Name: "internal/database", Cause: "API layer should not directly import database"},
				},
			},
			{
				Path: "internal",
				Prohibited: []ProhibitedPkg{
					{Name: "unsafe", Cause: "Unsafe package is not allowed"},
				},
			},
		},
	}
}

func TestNewFormatter(t *testing.T) {
	testCases := []struct {
		name        string
		format      OutputFormat
		shouldError bool
	}{
		{"JSON formatter", FormatJSON, false},
		{"SARIF formatter", FormatSARIF, false},
		{"Checkstyle formatter", FormatCheckstyle, false},
		{"JUnit formatter", FormatJUnit, false},
		{"Markdown formatter", FormatMarkdown, false},
		{"Text formatter", FormatText, false},
		{"Invalid formatter", "invalid", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			formatter, err := NewFormatter(tc.format)
			if tc.shouldError {
				assert.Error(t, err)
				assert.Nil(t, formatter)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, formatter)
			}
		})
	}
}

func TestJSONFormatter(t *testing.T) {
	formatter := &JSONFormatter{Pretty: true}
	violations := createTestViolations()
	cfg := createTestConfig()

	output, err := formatter.Format(violations, cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, output)
	assert.Equal(t, "application/json", formatter.ContentType())

	// Parse JSON to verify structure
	var jsonOutput JSONOutput
	err = json.Unmarshal(output, &jsonOutput)
	require.NoError(t, err)

	assert.Equal(t, 3, jsonOutput.Summary.TotalViolations)
	assert.Equal(t, "failed", jsonOutput.Summary.Status)
	assert.Len(t, jsonOutput.Violations, 3)
	assert.NotEmpty(t, jsonOutput.Timestamp)

	// Check first violation
	firstViolation := jsonOutput.Violations[0]
	assert.Equal(t, "internal/api/handler.go", firstViolation.File)
	assert.Equal(t, "internal/database", firstViolation.Import)
	assert.Equal(t, "api-no-db", firstViolation.Rule)
	assert.NotEmpty(t, firstViolation.Cause)
}

func TestJSONFormatterEmpty(t *testing.T) {
	formatter := &JSONFormatter{}
	violations := &LintViolations{Violations: []LintViolation{}}
	cfg := createTestConfig()

	output, err := formatter.Format(violations, cfg)
	require.NoError(t, err)

	var jsonOutput JSONOutput
	err = json.Unmarshal(output, &jsonOutput)
	require.NoError(t, err)

	assert.Equal(t, 0, jsonOutput.Summary.TotalViolations)
	assert.Equal(t, "passed", jsonOutput.Summary.Status)
}

func TestSARIFFormatter(t *testing.T) {
	formatter := &SARIFFormatter{}
	violations := createTestViolations()
	cfg := createTestConfig()

	output, err := formatter.Format(violations, cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, output)
	assert.Equal(t, "application/sarif+json", formatter.ContentType())

	// Parse SARIF to verify structure
	var sarifOutput SARIFOutput
	err = json.Unmarshal(output, &sarifOutput)
	require.NoError(t, err)

	assert.Equal(t, "2.1.0", sarifOutput.Version)
	assert.Contains(t, sarifOutput.Schema, "sarif-schema-2.1.0.json")
	assert.Len(t, sarifOutput.Runs, 1)

	run := sarifOutput.Runs[0]
	assert.Equal(t, "goverhaul", run.Tool.Driver.Name)
	assert.Len(t, run.Results, 3)

	// Check first result
	firstResult := run.Results[0]
	assert.Equal(t, "api-no-db", firstResult.RuleID)
	assert.Equal(t, "error", firstResult.Level)
	assert.Contains(t, firstResult.Message.Text, "internal/database")
	assert.Len(t, firstResult.Locations, 1)
	assert.Equal(t, "internal/api/handler.go", firstResult.Locations[0].PhysicalLocation.ArtifactLocation.URI)
}

func TestCheckstyleFormatter(t *testing.T) {
	formatter := &CheckstyleFormatter{}
	violations := createTestViolations()
	cfg := createTestConfig()

	output, err := formatter.Format(violations, cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, output)
	assert.Equal(t, "application/xml", formatter.ContentType())

	// Parse XML to verify structure
	var checkstyleOutput CheckstyleOutput
	err = xml.Unmarshal(output, &checkstyleOutput)
	require.NoError(t, err)

	assert.Equal(t, "8.0", checkstyleOutput.Version)
	assert.Len(t, checkstyleOutput.Files, 2) // Two unique files

	// Find the handler.go file
	var handlerFile *CheckstyleFile
	for _, f := range checkstyleOutput.Files {
		if f.Name == "internal/api/handler.go" {
			handlerFile = &f
			break
		}
	}
	require.NotNil(t, handlerFile)
	assert.Len(t, handlerFile.Errors, 2) // Two violations in handler.go
}

func TestJUnitFormatter(t *testing.T) {
	formatter := &JUnitFormatter{}
	violations := createTestViolations()
	cfg := createTestConfig()

	output, err := formatter.Format(violations, cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, output)
	assert.Equal(t, "application/xml", formatter.ContentType())

	// Parse XML to verify structure
	var junitOutput JUnitTestSuites
	err = xml.Unmarshal(output, &junitOutput)
	require.NoError(t, err)

	assert.Equal(t, "Goverhaul Architecture Tests", junitOutput.Name)
	assert.Equal(t, 3, junitOutput.Tests)
	assert.Equal(t, 3, junitOutput.Failures)
	assert.Len(t, junitOutput.TestSuites, 2) // Two unique files

	// Check first test suite
	firstSuite := junitOutput.TestSuites[0]
	assert.NotEmpty(t, firstSuite.Name)
	assert.Greater(t, firstSuite.Tests, 0)
	assert.Greater(t, firstSuite.Failures, 0)
}

func TestMarkdownFormatter(t *testing.T) {
	formatter := &MarkdownFormatter{}
	violations := createTestViolations()
	cfg := createTestConfig()

	output, err := formatter.Format(violations, cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, output)
	assert.Equal(t, "text/markdown", formatter.ContentType())

	content := string(output)
	assert.Contains(t, content, "# Goverhaul Architecture Violations Report")
	assert.Contains(t, content, "## Summary")
	assert.Contains(t, content, "## Violations by Rule")
	assert.Contains(t, content, "## Violations by File")
	assert.Contains(t, content, "**Total Violations:** 3")
	assert.Contains(t, content, "**Files with Issues:** 2")
	assert.Contains(t, content, "internal/api/handler.go")
	assert.Contains(t, content, "internal/database")
}

func TestMarkdownFormatterEmpty(t *testing.T) {
	formatter := &MarkdownFormatter{}
	violations := &LintViolations{Violations: []LintViolation{}}
	cfg := createTestConfig()

	output, err := formatter.Format(violations, cfg)
	require.NoError(t, err)

	content := string(output)
	assert.Contains(t, content, "✅ **No violations found!**")
	assert.Contains(t, content, "Your codebase complies with all architectural rules")
}

func TestTextFormatter(t *testing.T) {
	formatter := &TextFormatter{}
	violations := createTestViolations()
	cfg := createTestConfig()

	output, err := formatter.Format(violations, cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, output)
	assert.Equal(t, "text/plain", formatter.ContentType())

	content := string(output)
	assert.Contains(t, content, "Found 3 rule violations")
	assert.Contains(t, content, "internal/api/handler.go")
}

func BenchmarkFormatters(b *testing.B) {
	violations := &LintViolations{
		Violations: make([]LintViolation, 100),
	}
	for i := 0; i < 100; i++ {
		violations.Violations[i] = LintViolation{
			File:   fmt.Sprintf("file%d.go", i),
			Import: fmt.Sprintf("import%d", i),
			Rule:   fmt.Sprintf("rule%d", i%10),
			Cause:  "Test violation",
		}
	}
	cfg := createTestConfig()

	formatters := []struct {
		name      string
		formatter Formatter
	}{
		{"JSON", &JSONFormatter{}},
		{"SARIF", &SARIFFormatter{}},
		{"Checkstyle", &CheckstyleFormatter{}},
		{"JUnit", &JUnitFormatter{}},
		{"Markdown", &MarkdownFormatter{}},
		{"Text", &TextFormatter{}},
	}

	for _, f := range formatters {
		b.Run(f.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := f.formatter.Format(violations, cfg)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestFormatterContentTypes(t *testing.T) {
	testCases := []struct {
		formatter   Formatter
		contentType string
	}{
		{&JSONFormatter{}, "application/json"},
		{&SARIFFormatter{}, "application/sarif+json"},
		{&CheckstyleFormatter{}, "application/xml"},
		{&JUnitFormatter{}, "application/xml"},
		{&MarkdownFormatter{}, "text/markdown"},
		{&TextFormatter{}, "text/plain"},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.contentType, tc.formatter.ContentType())
	}
}

func TestJSONFormatterRuleSummary(t *testing.T) {
	formatter := &JSONFormatter{Pretty: true}
	violations := createTestViolations()
	cfg := createTestConfig()

	output, err := formatter.Format(violations, cfg)
	require.NoError(t, err)

	var jsonOutput JSONOutput
	err = json.Unmarshal(output, &jsonOutput)
	require.NoError(t, err)

	// Check rule summaries
	assert.NotEmpty(t, jsonOutput.Rules)

	// Find specific rules
	ruleMap := make(map[string]int)
	for _, rule := range jsonOutput.Rules {
		ruleMap[rule.Name] = rule.Violations
	}

	assert.Equal(t, 1, ruleMap["api-no-db"])
	assert.Equal(t, 1, ruleMap["no-unsafe"])
	assert.Equal(t, 1, ruleMap["service-no-api"])
}

func TestSARIFFormatterRules(t *testing.T) {
	formatter := &SARIFFormatter{}
	violations := createTestViolations()
	cfg := createTestConfig()

	output, err := formatter.Format(violations, cfg)
	require.NoError(t, err)

	var sarifOutput SARIFOutput
	err = json.Unmarshal(output, &sarifOutput)
	require.NoError(t, err)

	driver := sarifOutput.Runs[0].Tool.Driver
	assert.Len(t, driver.Rules, 3) // Three unique rules

	// Check that all rule IDs are present
	ruleIDs := make(map[string]bool)
	for _, rule := range driver.Rules {
		ruleIDs[rule.ID] = true
		assert.NotEmpty(t, rule.ShortDescription.Text)
		assert.NotEmpty(t, rule.FullDescription.Text)
		assert.Equal(t, "error", rule.DefaultConfig.Level)
	}

	assert.True(t, ruleIDs["api-no-db"])
	assert.True(t, ruleIDs["no-unsafe"])
	assert.True(t, ruleIDs["service-no-api"])
}

// Test helper for XML formatting
func TestXMLValidation(t *testing.T) {
	violations := createTestViolations()
	cfg := createTestConfig()

	t.Run("Checkstyle XML", func(t *testing.T) {
		formatter := &CheckstyleFormatter{}
		output, err := formatter.Format(violations, cfg)
		require.NoError(t, err)

		// Verify it's valid XML
		var result CheckstyleOutput
		err = xml.Unmarshal(output, &result)
		assert.NoError(t, err)
	})

	t.Run("JUnit XML", func(t *testing.T) {
		formatter := &JUnitFormatter{}
		output, err := formatter.Format(violations, cfg)
		require.NoError(t, err)

		// Verify it's valid XML
		var result JUnitTestSuites
		err = xml.Unmarshal(output, &result)
		assert.NoError(t, err)
	})
}