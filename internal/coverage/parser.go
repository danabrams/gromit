package coverage

import (
	"strings"
	"unicode"
)

// Criterion represents a single acceptance criterion from a spec.
type Criterion struct {
	Number int
	Text   string
}

const (
	acceptanceCriteriaHeader = "## Acceptance Criteria"
	nextSectionPrefix        = "\n##"
)

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
		text, ok := parseListItem(trimmed)
		if ok {
			criteria = append(criteria, Criterion{Number: len(criteria) + 1, Text: text})
			lastCriterionIndex = len(criteria) - 1
			continue
		}

		// Only append continuation text to an existing criterion for indented lines.
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

	nextIdx := strings.Index(rest, nextSectionPrefix)
	if nextIdx != -1 {
		rest = rest[:nextIdx]
	}
	return rest, true
}

func isIndented(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}

func parseListItem(line string) (string, bool) {
	if line == "" {
		return "", false
	}

	switch line[0] {
	case '-', '*':
		return parseItemText(line[1:])
	}

	i := 0
	for i < len(line) && unicode.IsDigit(rune(line[i])) {
		i++
	}
	if i == 0 || i >= len(line) {
		return "", false
	}
	if line[i] != '.' && line[i] != ')' {
		return "", false
	}

	return parseItemText(line[i+1:])
}

func parseItemText(raw string) (string, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", false
	}
	return text, true
}
