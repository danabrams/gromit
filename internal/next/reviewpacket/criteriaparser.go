package reviewpacket

import (
	"regexp"
	"strings"
)

// numberedCriterionRe matches lines starting with "N." (e.g., "1.", "23.").
var numberedCriterionRe = regexp.MustCompile(`^\d+\.\s`)

// criterionPrefixRe matches the leading "N. " prefix to strip it.
var criterionPrefixRe = regexp.MustCompile(`^\d+\.\s+`)

// ParsedCriterion represents a single acceptance criterion extracted from markdown.
type ParsedCriterion struct {
	Text string
}

// ParseAcceptanceCriteria extracts numbered acceptance criteria from the
// ## Acceptance Criteria section of markdown content.
func ParseAcceptanceCriteria(content string) []ParsedCriterion {
	var criteria []ParsedCriterion

	lines := strings.Split(content, "\n")

	// Find "## Acceptance Criteria" section
	var criteriaIdx int = -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "## Acceptance Criteria") {
			criteriaIdx = i
			break
		}
	}

	if criteriaIdx < 0 {
		return criteria
	}

	// Parse numbered criteria starting after the section header
	i := criteriaIdx + 1
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Stop if we hit another ## section
		if strings.HasPrefix(trimmed, "##") && !strings.HasPrefix(trimmed, "###") {
			break
		}

		// Check if this line starts with a number and period
		if isNumberedCriterion(trimmed) {
			criterion := ParsedCriterion{
				Text: extractCriterionText(trimmed),
			}
			criteria = append(criteria, criterion)
		}

		i++
	}

	return criteria
}

// isNumberedCriterion checks if a line starts with "N." pattern.
func isNumberedCriterion(line string) bool {
	return numberedCriterionRe.MatchString(line)
}

// extractCriterionText extracts the text after "N." from a numbered criterion line.
func extractCriterionText(line string) string {
	text := criterionPrefixRe.ReplaceAllString(line, "")
	return strings.TrimSpace(text)
}
