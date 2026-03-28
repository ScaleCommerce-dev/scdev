package ui

import (
	"regexp"
	"strings"
)

// ANSI codes for terminal styling
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiCyan   = "\x1b[36m"
	ansiYellow = "\x1b[33m"
	ansiGreen  = "\x1b[32m"
)

// RenderMarkdown converts simple markdown to ANSI-styled terminal output.
// Supports: headings (#, ##, ###), code blocks (```), inline code (`), bold (**)
// Unlike glamour, this actually removes the markdown syntax for cleaner output.
func RenderMarkdown(input string, plainMode bool) string {
	if plainMode {
		return input
	}

	var result strings.Builder
	lines := strings.Split(input, "\n")
	inCodeBlock := false

	// Pre-compile regex patterns
	inlineCodeRe := regexp.MustCompile("`([^`]+)`")
	boldRe := regexp.MustCompile(`\*\*([^*]+)\*\*`)

	for _, line := range lines {
		// Handle code blocks
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}

		if inCodeBlock {
			// Code block content - cyan and indented
			result.WriteString("  " + ansiCyan + line + ansiReset + "\n")
			continue
		}

		// Handle headings - remove markers and make bold/colored
		if strings.HasPrefix(line, "### ") {
			heading := strings.TrimPrefix(line, "### ")
			result.WriteString(ansiBold + ansiYellow + heading + ansiReset + "\n")
			continue
		}
		if strings.HasPrefix(line, "## ") {
			heading := strings.TrimPrefix(line, "## ")
			result.WriteString(ansiBold + ansiGreen + heading + ansiReset + "\n")
			continue
		}
		if strings.HasPrefix(line, "# ") {
			heading := strings.TrimPrefix(line, "# ")
			result.WriteString(ansiBold + ansiGreen + heading + ansiReset + "\n")
			continue
		}

		// Handle inline code `code` - cyan, remove backticks
		line = inlineCodeRe.ReplaceAllString(line, ansiCyan+"$1"+ansiReset)

		// Handle bold **text** - bold, remove asterisks
		line = boldRe.ReplaceAllString(line, ansiBold+"$1"+ansiReset)

		result.WriteString(line + "\n")
	}

	return result.String()
}
