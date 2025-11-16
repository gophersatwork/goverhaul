package goverhaul

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/afero"
)

// CodeContext represents source code lines around a violation
type CodeContext struct {
	Lines         []CodeLine // Source lines with context
	ViolationLine int        // Which line has the violation (1-indexed)
}

// CodeLine represents a single line of source code
type CodeLine struct {
	Number      int    // Line number (1-indexed)
	Content     string // Line content
	IsViolation bool   // True if this is the violation line
}

// ExtractCodeContext extracts source code around a violation position
// Shows N lines before and after (default 2)
func ExtractCodeContext(fs afero.Fs, filePath string, position *Position, contextLines int) (*CodeContext, error) {
	if position == nil || !position.IsValid() {
		return nil, nil // No position, no context
	}

	file, err := fs.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	lineNum := 1

	// Read all lines into memory
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Calculate context range
	startLine := max(1, position.Line-contextLines)
	endLine := min(len(lines), position.Line+contextLines)

	// Build context
	var codeLines []CodeLine
	for i := startLine; i <= endLine; i++ {
		codeLines = append(codeLines, CodeLine{
			Number:      i,
			Content:     lines[i-1], // 0-indexed array
			IsViolation: i == position.Line,
		})
	}

	return &CodeContext{
		Lines:         codeLines,
		ViolationLine: position.Line,
	}, nil
}

// Format formats code context with line numbers and violation markers
func (c *CodeContext) Format(useColor bool) string {
	if c == nil || len(c.Lines) == 0 {
		return ""
	}

	var result strings.Builder
	maxLineNum := c.Lines[len(c.Lines)-1].Number
	width := len(fmt.Sprintf("%d", maxLineNum))

	for _, line := range c.Lines {
		marker := " "
		lineColor := ""
		resetColor := ""

		if line.IsViolation {
			marker = ">"
			if useColor {
				// ANSI red color for violation line
				lineColor = "\033[31m"
				resetColor = "\033[0m"
			}
		}

		lineNumStr := fmt.Sprintf("%*d", width, line.Number)
		result.WriteString(fmt.Sprintf("%s%s %s | %s%s\n",
			lineColor, marker, lineNumStr, line.Content, resetColor))
	}

	return result.String()
}

// CodeContextCache caches file contents to avoid re-reading for multiple violations
type CodeContextCache struct {
	fs    afero.Fs
	cache map[string][]string
}

// NewCodeContextCache creates a new code context cache
func NewCodeContextCache(fs afero.Fs) *CodeContextCache {
	return &CodeContextCache{
		fs:    fs,
		cache: make(map[string][]string),
	}
}

// GetContext extracts code context using cached file contents
func (cc *CodeContextCache) GetContext(filePath string, position *Position, contextLines int) (*CodeContext, error) {
	if position == nil || !position.IsValid() {
		return nil, nil
	}

	// Check cache first
	lines, exists := cc.cache[filePath]
	if !exists {
		// Load file into cache
		file, err := cc.fs.Open(filePath)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lines = []string{}

		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			return nil, err
		}

		cc.cache[filePath] = lines
	}

	// Calculate context range
	startLine := max(1, position.Line-contextLines)
	endLine := min(len(lines), position.Line+contextLines)

	// Build context
	var codeLines []CodeLine
	for i := startLine; i <= endLine; i++ {
		codeLines = append(codeLines, CodeLine{
			Number:      i,
			Content:     lines[i-1], // 0-indexed array
			IsViolation: i == position.Line,
		})
	}

	return &CodeContext{
		Lines:         codeLines,
		ViolationLine: position.Line,
	}, nil
}

// Helper functions for min/max (Go 1.21+)
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
