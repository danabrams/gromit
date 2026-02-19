package coverage

import (
	"fmt"
	"regexp"
	"strings"
)

// Criterion represents a single acceptance criterion from a spec.
type Criterion struct {
	Number int
	Text   string
}

var bulletRe = regexp.MustCompile(`^-\s+AC(\d+):\s+(.+)$`)

// ParseCriteria extracts acceptance criteria from the "## Acceptance Criteria"
// section of a spec document.
func ParseCriteria(specContent string) ([]Criterion, error) {
	section, err := extractSection(specContent, "## Acceptance Criteria")
	if err != nil {
		return nil, err
	}

	var criteria []Criterion
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		m := bulletRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		var n int
		if _, err := fmt.Sscan(m[1], &n); err != nil {
			continue
		}
		criteria = append(criteria, Criterion{Number: n, Text: m[2]})
	}

	if criteria == nil {
		criteria = []Criterion{}
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
