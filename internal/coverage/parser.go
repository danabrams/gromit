package coverage

import (
	"fmt"
	"strings"
)

// Criterion represents a single acceptance criterion from a spec.
type Criterion struct {
	Number int
	Text   string
}

// ParseCriteria extracts acceptance criteria from the "## Acceptance Criteria"
// section of a spec document.
func ParseCriteria(specContent string) ([]Criterion, error) {
	section, err := extractSection(specContent, "## Acceptance Criteria")
	if err != nil {
		return nil, err
	}

	criteria := make([]Criterion, 0)
	currentIdx := -1
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-") {
			text := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			if text == "" {
				currentIdx = -1
				continue
			}
			criteria = append(criteria, Criterion{Number: len(criteria) + 1, Text: text})
			currentIdx = len(criteria) - 1
			continue
		}
		if currentIdx != -1 && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
			cont := strings.TrimSpace(line)
			if cont != "" {
				criteria[currentIdx].Text += " " + cont
			}
		}
	}
	return criteria, nil
}

// extractSection returns the content after the given header until the next
// section header (##) or end of string.
func extractSection(content, header string) (string, error) {
	idx := strings.Index(content, header)
	if idx == -1 {
		return "", fmt.Errorf("section %q not found", header)
	}
	rest := content[idx+len(header):]

	// Find next ## heading
	nextIdx := strings.Index(rest, "\n##")
	if nextIdx != -1 {
		rest = rest[:nextIdx]
	}
	return rest, nil
}
