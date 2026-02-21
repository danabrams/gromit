package validate

import (
	"fmt"
	"sort"
	"strings"
)

// BuildReprompt builds a focused re-decompose prompt after validation failures.
func BuildReprompt(originalPrompt string, candidates []BeadCandidate, violations []Violation) string {
	var sb strings.Builder

	sb.WriteString("The previous decomposition output failed validation.\n")
	sb.WriteString("Revise only the flagged beads to address the listed violations.\n\n")

	if originalPrompt != "" {
		sb.WriteString("## Original Prompt\n\n")
		sb.WriteString(originalPrompt)
		if !strings.HasSuffix(originalPrompt, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Candidate Beads\n\n")
	for i, c := range candidates {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i, c.Title))
		sb.WriteString(fmt.Sprintf("   Description: %s\n", c.Description))
		sb.WriteString("   Acceptance Criteria:\n")
		for _, criterion := range c.AcceptanceCriteria {
			sb.WriteString(fmt.Sprintf("   - %s\n", criterion))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Violations By Flagged Bead\n\n")
	groupedViolations := make(map[int][]Violation)
	for _, v := range violations {
		groupedViolations[v.BeadIndex] = append(groupedViolations[v.BeadIndex], v)
	}
	indexes := make([]int, 0, len(groupedViolations))
	for beadIndex := range groupedViolations {
		indexes = append(indexes, beadIndex)
	}
	sort.Ints(indexes)

	for _, beadIndex := range indexes {
		sb.WriteString(fmt.Sprintf("- Bead %d:\n", beadIndex))
		for _, v := range groupedViolations[beadIndex] {
			sb.WriteString(fmt.Sprintf("  - [%s] %s\n", v.Rule, v.Message))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## Instructions\n\n")
	sb.WriteString("- Keep every unflagged bead unchanged (title, description, depends_on, and acceptance_criteria).\n")
	sb.WriteString("- Modify only flagged beads, and only as needed to fix the listed validation violations.\n")
	sb.WriteString("- Return the same JSON format as before: a JSON array of bead objects with title, description, depends_on, and acceptance_criteria.\n")
	sb.WriteString("- Respond with ONLY the JSON array (no markdown, no explanation).\n")

	return sb.String()
}
