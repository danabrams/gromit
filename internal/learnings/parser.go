package learnings

import (
	"strings"
	"time"
)

// parseLearnings parses the LEARNINGS.md file content.
func parseLearnings(content string) (confirmed, provisional, archived []Learning) {
	// Simple parser - looks for ### headers in Confirmed/Provisional/Archived sections.
	lines := strings.Split(content, "\n")

	inConfirmed := false
	inProvisional := false
	inArchived := false
	var current *Learning
	var contentLines []string

	saveCurrent := func() {
		if current != nil && len(contentLines) > 0 {
			current.Content = strings.TrimSpace(strings.Join(contentLines, "\n"))
			current.Hash = hashContent(current.Content)
			if inConfirmed {
				confirmed = append(confirmed, *current)
			} else if inProvisional {
				provisional = append(provisional, *current)
			} else if inArchived {
				archived = append(archived, *current)
			}
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "## Confirmed") {
			saveCurrent()
			current = nil
			contentLines = nil
			inConfirmed = true
			inProvisional = false
			inArchived = false
			continue
		}
		if strings.HasPrefix(line, "## Provisional") {
			saveCurrent()
			current = nil
			contentLines = nil
			inConfirmed = false
			inProvisional = true
			inArchived = false
			continue
		}
		if strings.HasPrefix(line, "## Archived") {
			saveCurrent()
			current = nil
			contentLines = nil
			inConfirmed = false
			inProvisional = false
			inArchived = true
			continue
		}

		if strings.HasPrefix(line, "### ") {
			// Save previous learning.
			saveCurrent()

			// Parse header: ### 2026-02-05 | bead-id | category.
			parts := strings.Split(strings.TrimPrefix(line, "### "), " | ")
			current = &Learning{}
			contentLines = nil

			if len(parts) >= 1 {
				current.Date, _ = time.Parse("2006-01-02", strings.TrimSpace(parts[0]))
			}
			if len(parts) >= 2 {
				current.BeadID = strings.TrimSpace(parts[1])
			}
			if len(parts) >= 3 {
				current.Category = strings.TrimSpace(parts[2])
			}
			continue
		}

		if current != nil {
			// Check for related-to line.
			if strings.HasPrefix(line, "*Related to:") {
				if matches := relatedToRegex.FindStringSubmatch(line); len(matches) > 1 {
					current.RelatedTo = matches[1]
				}
				continue
			}
			// Skip section dividers.
			if strings.TrimSpace(line) == "---" {
				continue
			}
			// Include everything else, including *Archived from* lines
			// (those are part of the content, not metadata like RelatedTo).
			contentLines = append(contentLines, line)
		}
	}

	// Don't forget the last learning.
	saveCurrent()

	return confirmed, provisional, archived
}
