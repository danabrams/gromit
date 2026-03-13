package acceptor

import (
	"fmt"
	"strings"
)

// ParseAcceptanceCriteria extracts acceptance criteria from a markdown spec.
// It looks for a "## Acceptance Criteria" section and collects bullet points
// until the next ## heading or EOF. Returns error if the section is not found.
func ParseAcceptanceCriteria(specMarkdown string) ([]string, error) {
	lines := strings.Split(specMarkdown, "\n")
	inSection := false
	var criteria []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if inSection {
			// Stop at next ## heading
			if strings.HasPrefix(trimmed, "## ") {
				break
			}
			// Parse bullet points (-, *, numbered)
			if bullet, ok := parseBullet(trimmed); ok {
				// Only top-level bullets (not indented)
				if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
					criteria = append(criteria, bullet)
				}
			}
			continue
		}

		// Look for the acceptance criteria heading (exact match per design)
		if strings.HasPrefix(trimmed, "## ") {
			heading := strings.TrimPrefix(trimmed, "## ")
			if strings.TrimSpace(heading) == "Acceptance Criteria" {
				inSection = true
			}
		}
	}

	if !inSection {
		return nil, fmt.Errorf("no '## Acceptance Criteria' section found in spec")
	}

	if criteria == nil {
		criteria = []string{}
	}
	return criteria, nil
}

// parseBullet extracts text from a bullet line. Returns the text and true if
// the line is a bullet, or empty string and false otherwise.
func parseBullet(line string) (string, bool) {
	if strings.HasPrefix(line, "- ") {
		return strings.TrimSpace(strings.TrimPrefix(line, "- ")), true
	}
	if strings.HasPrefix(line, "* ") {
		return strings.TrimSpace(strings.TrimPrefix(line, "* ")), true
	}
	// Numbered: "1. ", "2. ", etc.
	for i, ch := range line {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch == '.' && i > 0 && i+1 < len(line) && line[i+1] == ' ' {
			return strings.TrimSpace(line[i+2:]), true
		}
		break
	}
	return "", false
}
