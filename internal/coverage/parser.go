package coverage

import (
	"errors"
	"fmt"
	"strings"
)

// Criterion represents a single acceptance criterion from a spec.
type Criterion struct {
	Number int
	Text   string
}

var errCriteriaSectionNotFound = errors.New("acceptance criteria section not found")

const acceptanceCriteriaHeader = "## Acceptance Criteria"

// ParseCriteria extracts acceptance criteria from the "## Acceptance Criteria"
// section of a spec document.
func ParseCriteria(specContent string) ([]Criterion, error) {
	section, err := extractSection(specContent, acceptanceCriteriaHeader)
	if err != nil {
		return nil, err
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
func extractSection(content, header string) (string, error) {
	idx := strings.Index(content, header)
	if idx == -1 {
		return "", fmt.Errorf("%w: %s", errCriteriaSectionNotFound, header)
	}
	rest := content[idx+len(header):]

	// Find next ## heading
	nextIdx := strings.Index(rest, "\n##")
	if nextIdx != -1 {
		rest = rest[:nextIdx]
	}
	return rest, nil
}

func isIndented(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}
