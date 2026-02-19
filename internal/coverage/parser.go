package coverage

import (
	"fmt"
	"regexp"
	"strconv"
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

	criteria := make([]Criterion, 0)
	var current *Criterion
	finishCurrent := func() {
		if current != nil {
			criteria = append(criteria, *current)
		}
	}
	for _, line := range strings.Split(section, "\n") {
		m := bulletRe.FindStringSubmatch(strings.TrimSpace(line))
		if m != nil {
			finishCurrent()
			n, err := strconv.Atoi(m[1])
			if err != nil {
				current = nil
				continue
			}
			c := Criterion{Number: n, Text: m[2]}
			current = &c
			continue
		}
		// Continuation line: indented non-empty text appended to current criterion.
		if current != nil && len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			cont := strings.TrimSpace(line)
			if cont != "" {
				current.Text += " " + cont
			}
		}
	}
	finishCurrent()
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
