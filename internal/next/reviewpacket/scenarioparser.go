package reviewpacket

import (
	"regexp"
	"strings"
)

// whitespaceRe collapses multiple whitespace characters into a single space.
var whitespaceRe = regexp.MustCompile(`\s+`)

// ParsedScenario represents a single scenario extracted from markdown.
type ParsedScenario struct {
	Title string
	Given string
	When  string
	Then  string
	Notes string
}

// ParseScenarios extracts all ### Scenario: blocks from markdown content
// and returns their Given/When/Then lines.
func ParseScenarios(content string) []ParsedScenario {
	var scenarios []ParsedScenario

	// Split content into lines
	lines := strings.Split(content, "\n")

	var i int
	for i < len(lines) {
		line := lines[i]

		// Check if this is a scenario header
		if strings.HasPrefix(strings.TrimSpace(line), "### Scenario:") {
			scenario := parseScenario(lines, &i)
			scenarios = append(scenarios, scenario)
		} else {
			i++
		}
	}

	return scenarios
}

// parseScenario parses a single scenario starting at current line index.
// It updates the index pointer to move past the scenario.
func parseScenario(lines []string, idx *int) ParsedScenario {
	scenario := ParsedScenario{}

	// Extract title from "### Scenario: Title" line
	line := strings.TrimSpace(lines[*idx])
	title := strings.TrimPrefix(line, "### Scenario:")
	scenario.Title = strings.TrimSpace(title)

	*idx++

	// Look for Given, When, Then lines
	for *idx < len(lines) {
		line := lines[*idx]
		trimmed := strings.TrimSpace(line)

		// Stop if we hit another scenario
		if strings.HasPrefix(trimmed, "### Scenario:") {
			break
		}

		// Stop if we hit another header that's not a continuation
		if strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "###") {
			break
		}

		// Parse Given/When/Then keywords (case-insensitive)
		lowerTrimmed := strings.ToLower(trimmed)

		if strings.HasPrefix(lowerTrimmed, "given ") {
			scenario.Given = extractKeywordContent(trimmed, "given")
		} else if strings.HasPrefix(lowerTrimmed, "when ") {
			scenario.When = extractKeywordContent(trimmed, "when")
		} else if strings.HasPrefix(lowerTrimmed, "then ") {
			scenario.Then = extractKeywordContent(trimmed, "then")
		} else if strings.HasPrefix(lowerTrimmed, "**notes:**") {
			scenario.Notes = extractNotesContent(trimmed)
		}

		*idx++
	}

	return scenario
}

// extractKeywordContent extracts the content after a Given/When/Then keyword.
// It handles case-insensitive matching and normalizes whitespace.
func extractKeywordContent(line, keyword string) string {
	// Find the keyword (case-insensitive)
	lowerLine := strings.ToLower(line)
	keywordWithSpace := keyword + " "
	idx := strings.Index(lowerLine, keywordWithSpace)

	if idx < 0 {
		return ""
	}

	// Extract content after keyword + space
	content := line[idx+len(keywordWithSpace):]
	content = strings.TrimSpace(content)

	// Normalize whitespace: collapse multiple spaces to single space
	content = whitespaceRe.ReplaceAllString(content, " ")

	return content
}

// extractNotesContent extracts the content after a **Notes:** marker.
func extractNotesContent(line string) string {
	lowerLine := strings.ToLower(line)
	idx := strings.Index(lowerLine, "**notes:**")
	if idx < 0 {
		return ""
	}
	content := line[idx+len("**notes:**"):]
	content = strings.TrimSpace(content)
	content = whitespaceRe.ReplaceAllString(content, " ")
	return content
}
