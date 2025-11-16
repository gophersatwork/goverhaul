package goverhaul

// This file contains the enhanced text formatter with color support
// It's added as a separate file to avoid modification conflicts

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
)

// ColorMode represents when to use colors in output
type ColorMode string

const (
	// ColorAuto automatically detects TTY and enables colors appropriately
	ColorAuto ColorMode = "auto"
	// ColorAlways forces colors to be enabled
	ColorAlways ColorMode = "always"
	// ColorNever disables colors
	ColorNever ColorMode = "never"
)

// EnhancedTextFormatter outputs violations with beautiful ANSI colors
type EnhancedTextFormatter struct {
	// ColorMode controls when to enable colors (auto, always, never)
	ColorMode ColorMode
	// GroupByRule when true groups violations by rule instead of file
	GroupByRule bool
	// Writer is the output destination (defaults to os.Stdout)
	Writer io.Writer
}

// NewTextFormatter creates a new EnhancedTextFormatter with sensible defaults
func NewTextFormatter() *EnhancedTextFormatter {
	return &EnhancedTextFormatter{
		ColorMode:   ColorAuto,
		GroupByRule: false,
		Writer:      os.Stdout,
	}
}

func (f *EnhancedTextFormatter) Format(violations *LintViolations, cfg *Config) ([]byte, error) {
	// Determine if colors should be enabled
	enableColor := f.shouldEnableColor()

	var sb strings.Builder

	if enableColor {
		sb.WriteString(f.formatWithColors(violations))
	} else {
		// Fallback to plain text
		if f.GroupByRule {
			sb.WriteString(violations.PrintByRule())
		} else {
			sb.WriteString(violations.PrintByFile())
		}
	}

	return []byte(sb.String()), nil
}

func (f *EnhancedTextFormatter) ContentType() string {
	return "text/plain"
}

// shouldEnableColor determines if colors should be enabled based on the ColorMode
func (f *EnhancedTextFormatter) shouldEnableColor() bool {
	switch f.ColorMode {
	case ColorAlways:
		return true
	case ColorNever:
		return false
	case ColorAuto:
		// Auto-detect if output is a TTY
		writer := f.Writer
		if writer == nil {
			writer = os.Stdout
		}

		// Check if writer is a file and if it's a terminal
		if file, ok := writer.(*os.File); ok {
			fileInfo, err := file.Stat()
			if err != nil {
				return false
			}
			// Check if it's a character device (terminal)
			return (fileInfo.Mode() & os.ModeCharDevice) != 0
		}
		return false
	default:
		return false
	}
}

// formatWithColors creates a beautifully colored output
func (f *EnhancedTextFormatter) formatWithColors(violations *LintViolations) string {
	var sb strings.Builder

	// Color definitions
	errorColor := color.New(color.FgRed, color.Bold)
	warningColor := color.New(color.FgYellow, color.Bold)
	infoColor := color.New(color.FgBlue, color.Bold)
	hintColor := color.New(color.FgHiBlack, color.Bold)
	fileColor := color.New(color.FgCyan, color.Bold)
	importColor := color.New(color.FgMagenta)
	successColor := color.New(color.FgGreen, color.Bold)
	boxColor := color.New(color.FgHiBlack)
	ruleColor := color.New(color.FgYellow)

	// Header box
	boxTop := "╭─────────────────────────────────────────────────────╮"
	boxBottom := "╰─────────────────────────────────────────────────────╯"

	sb.WriteString(boxColor.Sprint(boxTop) + "\n")
	sb.WriteString(boxColor.Sprint("│") + "  " + color.New(color.Bold).Sprint("Goverhaul Architecture Linter") + strings.Repeat(" ", 22) + boxColor.Sprint("│") + "\n")

	// Check if there are no violations
	if violations.IsEmpty() {
		sb.WriteString(boxColor.Sprint("│") + "  " + successColor.Sprint("No violations found!") + strings.Repeat(" ", 31) + boxColor.Sprint("│") + "\n")
		sb.WriteString(boxColor.Sprint(boxBottom) + "\n")
		return sb.String()
	}

	// Count violations by severity
	errorCount, warningCount, infoCount, hintCount := f.countBySeverity(violations)
	totalCount := len(violations.Violations)

	// Count unique files
	fileMap := make(map[string]bool)
	for _, v := range violations.Violations {
		fileMap[v.File] = true
	}

	summaryText := fmt.Sprintf("Found %d violations in %d files", totalCount, len(fileMap))
	padding := 53 - len(summaryText) - 2
	if padding < 0 {
		padding = 0
	}
	sb.WriteString(boxColor.Sprint("│") + "  " + summaryText + strings.Repeat(" ", padding) + boxColor.Sprint("│") + "\n")
	sb.WriteString(boxColor.Sprint(boxBottom) + "\n\n")

	// Group and display violations
	if f.GroupByRule {
		f.formatByRule(&sb, violations, errorColor, warningColor, infoColor, hintColor, fileColor, importColor, ruleColor)
	} else {
		f.formatByFile(&sb, violations, errorColor, warningColor, infoColor, hintColor, fileColor, importColor, ruleColor)
	}

	// Summary footer
	sb.WriteString(boxColor.Sprint(boxTop) + "\n")

	summaryParts := make([]string, 0, 4)
	if errorCount > 0 {
		summaryParts = append(summaryParts, errorColor.Sprintf("%d errors", errorCount))
	}
	if warningCount > 0 {
		summaryParts = append(summaryParts, warningColor.Sprintf("%d warnings", warningCount))
	}
	if infoCount > 0 {
		summaryParts = append(summaryParts, infoColor.Sprintf("%d info", infoCount))
	}
	if hintCount > 0 {
		summaryParts = append(summaryParts, hintColor.Sprintf("%d hints", hintCount))
	}

	summaryLine := "Summary: " + strings.Join(summaryParts, ", ")
	// Strip ANSI codes for length calculation
	plainSummary := stripAnsiCodes(summaryLine)
	padding = 53 - len(plainSummary) - 2
	if padding < 0 {
		padding = 0
	}
	sb.WriteString(boxColor.Sprint("│") + "  " + summaryLine + strings.Repeat(" ", padding) + boxColor.Sprint("│") + "\n")
	sb.WriteString(boxColor.Sprint(boxBottom) + "\n")

	return sb.String()
}

// formatByFile groups violations by file
func (f *EnhancedTextFormatter) formatByFile(sb *strings.Builder, violations *LintViolations,
	errorColor, warningColor, infoColor, hintColor, fileColor, importColor, ruleColor *color.Color) {

	// Group violations by file
	fileViolations := make(map[string][]LintViolation)
	for _, v := range violations.Violations {
		fileViolations[v.File] = append(fileViolations[v.File], v)
	}

	// Display violations for each file
	for file, viols := range fileViolations {
		sb.WriteString(fileColor.Sprintf("📁 %s", file))
		sb.WriteString(color.HiBlackString(" (%d violations)", len(viols)))
		sb.WriteString("\n\n")

		for _, v := range viols {
			f.formatViolation(sb, &v, errorColor, warningColor, infoColor, hintColor, importColor, ruleColor)
		}

		sb.WriteString("\n")
	}
}

// formatByRule groups violations by rule
func (f *EnhancedTextFormatter) formatByRule(sb *strings.Builder, violations *LintViolations,
	errorColor, warningColor, infoColor, hintColor, fileColor, importColor, ruleColor *color.Color) {

	// Group violations by rule
	ruleViolations := make(map[string][]LintViolation)
	for _, v := range violations.Violations {
		ruleViolations[v.Rule] = append(ruleViolations[v.Rule], v)
	}

	// Display violations for each rule
	for rule, viols := range ruleViolations {
		sb.WriteString(ruleColor.Sprintf("📋 Rule: %s", rule))
		sb.WriteString(color.HiBlackString(" (%d violations)", len(viols)))
		sb.WriteString("\n\n")

		for _, v := range viols {
			f.formatViolation(sb, &v, errorColor, warningColor, infoColor, hintColor, importColor, ruleColor)
		}

		sb.WriteString("\n")
	}
}

// formatViolation formats a single violation with colors
func (f *EnhancedTextFormatter) formatViolation(sb *strings.Builder, v *LintViolation,
	errorColor, warningColor, infoColor, hintColor, importColor, ruleColor *color.Color) {

	// Get severity
	severity := v.Severity
	if severity == "" {
		severity = SeverityError // Default to error
	}

	// Choose icon and color based on severity
	var icon string
	var severityColor *color.Color

	switch severity {
	case SeverityError:
		icon = "❌"
		severityColor = errorColor
	case SeverityWarning:
		icon = "⚠️ "
		severityColor = warningColor
	case SeverityInfo:
		icon = "ℹ️ "
		severityColor = infoColor
	case SeverityHint:
		icon = "💡"
		severityColor = hintColor
	default:
		icon = "❌"
		severityColor = errorColor
	}

	// Format position if available
	position := ""
	if v.Position != nil && v.Position.IsValid() {
		position = fmt.Sprintf(" · line %d", v.Position.Line)
		if v.Position.Column > 0 {
			position += fmt.Sprintf(":%d", v.Position.Column)
		}
	}

	// Write violation line
	sb.WriteString("  ")
	sb.WriteString(icon)
	sb.WriteString(" import ")
	sb.WriteString(importColor.Sprintf("\"%s\"", v.Import))
	sb.WriteString(position)
	sb.WriteString("\n")

	// Write rule information
	sb.WriteString("     ")
	sb.WriteString(color.HiBlackString("Rule: "))
	sb.WriteString(ruleColor.Sprint(v.Rule))
	sb.WriteString("\n")

	// Write severity
	sb.WriteString("     ")
	sb.WriteString(color.HiBlackString("Severity: "))
	sb.WriteString(severityColor.Sprint(string(severity)))
	sb.WriteString("\n")

	// Write cause/message
	if v.Cause != "" {
		sb.WriteString("     ")
		sb.WriteString(v.Cause)
		sb.WriteString("\n")
	}

	// Write details if available
	if v.Details != "" {
		sb.WriteString("     ")
		sb.WriteString(color.HiBlackString("Details: "))
		sb.WriteString(v.Details)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
}

// countBySeverity counts violations by severity level
func (f *EnhancedTextFormatter) countBySeverity(violations *LintViolations) (errors, warnings, infos, hints int) {
	for _, v := range violations.Violations {
		severity := v.Severity
		if severity == "" {
			severity = SeverityError
		}

		switch severity {
		case SeverityError:
			errors++
		case SeverityWarning:
			warnings++
		case SeverityInfo:
			infos++
		case SeverityHint:
			hints++
		}
	}
	return
}

// stripAnsiCodes removes ANSI color codes from a string for length calculation
func stripAnsiCodes(s string) string {
	// Simple ANSI code stripper for length calculation
	inEscape := false
	var result strings.Builder

	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}

	return result.String()
}
