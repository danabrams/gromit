package coverage

import (
	"strings"
)

// Criterion represents a single acceptance criterion from a spec.
type Criterion struct {
	Number int
	Text   string
}

const acceptanceCriteriaHeader = "## Acceptance Criteria"

// ParseCriteria extracts acceptance criteria from the "## Acceptance Criteria"
// section of a spec document.
func ParseCriteria(specContent string) ([]Criterion, error) {
	section, ok := extractSection(specContent, acceptanceCriteriaHeader)
	if !ok {
		return []Criterion{}, nil
	}

	criteria := make([]Criterion, 0)
	lastCriterionIndex := -1
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-") {
			text := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			if text == "" {
				lastCriterionIndex = -1
				continue
			}
			criteria = append(criteria, Criterion{Number: len(criteria) + 1, Text: text})
			lastCriterionIndex = len(criteria) - 1
			continue
		}
		if lastCriterionIndex == -1 || !isIndented(line) {
			continue
		}
		cont := strings.TrimSpace(line)
		if cont != "" {
			criteria[lastCriterionIndex].Text += " " + cont
		}
	}
	return criteria, nil
}

// extractSection returns the content after the given header until the next
// section header (##) or end of string.
func extractSection(content, header string) (string, bool) {
	idx := strings.Index(content, header)
	if idx == -1 {
		return "", false
	}
	rest := content[idx+len(header):]

	// Find next ## heading
	nextIdx := strings.Index(rest, "\n##")
	if nextIdx != -1 {
		rest = rest[:nextIdx]
	}
	return rest, true
}

func isIndented(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}
