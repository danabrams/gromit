package reviewpacket

import (
	"regexp"
	"strings"
)

// numberedCriterionRe matches lines starting with "N." (e.g., "1.", "23.").
var numberedCriterionRe = regexp.MustCompile(`^\d+\.\s`)

// criterionPrefixRe matches the leading "N. " prefix to strip it.
var criterionPrefixRe = regexp.MustCompile(`^\d+\.\s+`)

// dashPrefixRe matches the leading "- " prefix to strip it.
var dashPrefixRe = regexp.MustCompile(`^-\s+`)

// ParsedCriterion represents a single acceptance criterion extracted from markdown.
type ParsedCriterion struct {
	Text string
}

// ParseAcceptanceCriteria extracts numbered (N.) and dash-prefixed (-) acceptance criteria
// from the ## Acceptance Criteria section of markdown content.
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

	// Parse criteria (numbered or dash-prefixed) starting after the section header
	i := criteriaIdx + 1
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Stop if we hit another ## section
		if strings.HasPrefix(trimmed, "##") && !strings.HasPrefix(trimmed, "###") {
			break
		}

		// Check if this line starts with a number and period or a dash (not indented)
		if isNumberedCriterion(trimmed) {
			criterion := ParsedCriterion{
				Text: extractNumberedCriterionText(trimmed),
			}
			criteria = append(criteria, criterion)
		} else if isDashCriterionAtRootLevel(line) {
			criterion := ParsedCriterion{
				Text: extractDashCriterionText(trimmed),
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

// isDashCriterionAtRootLevel checks if a line starts with "- " at the root level (not indented).
func isDashCriterionAtRootLevel(line string) bool {
	// Check if the line starts with "- " without leading spaces
	return strings.HasPrefix(line, "- ")
}

// extractNumberedCriterionText extracts the text after "N." from a numbered criterion line.
func extractNumberedCriterionText(line string) string {
	text := criterionPrefixRe.ReplaceAllString(line, "")
	return strings.TrimSpace(text)
}

// extractDashCriterionText extracts the text after "- " from a dash-prefixed criterion line.
func extractDashCriterionText(line string) string {
	text := dashPrefixRe.ReplaceAllString(line, "")
	return strings.TrimSpace(text)
}
