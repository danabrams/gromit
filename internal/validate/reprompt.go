package validate

import (
	"fmt"
	"sort"
	"strings"
)

const (
	repromptValidationFailureIntro = "The previous decomposition output failed validation.\n" +
		"Revise only the flagged beads to address the listed violations.\n\n"
	repromptOriginalPromptHeader = "## Original Prompt\n\n"
	repromptCandidatesHeader     = "## Candidate Beads\n\n"
	repromptViolationsHeader     = "## Violations By Flagged Bead\n\n"
	repromptInstructionsHeader   = "## Instructions\n\n"
	repromptInstructionsBody     = "- Keep every unflagged bead unchanged (title, description, depends_on, and acceptance_criteria).\n" +
		"- Modify only flagged beads, and only as needed to fix the listed validation violations.\n" +
		"- Return the same JSON format as before: a JSON array of bead objects with title, description, depends_on, and acceptance_criteria.\n" +
		"- Respond with ONLY the JSON array (no markdown, no explanation).\n"
)

// BuildReprompt builds a focused re-decompose prompt after validation failures.
func BuildReprompt(originalPrompt string, candidates []BeadCandidate, violations []Violation) string {
	var sb strings.Builder

	sb.WriteString(repromptValidationFailureIntro)

	if originalPrompt != "" {
		sb.WriteString(repromptOriginalPromptHeader)
		sb.WriteString(originalPrompt)
		if !strings.HasSuffix(originalPrompt, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(repromptCandidatesHeader)
	for i, c := range candidates {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i, c.Title))
		sb.WriteString(fmt.Sprintf("   Description: %s\n", c.Description))
		sb.WriteString("   Acceptance Criteria:\n")
		for _, criterion := range c.AcceptanceCriteria {
			sb.WriteString(fmt.Sprintf("   - %s\n", criterion))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(repromptViolationsHeader)
	groupedViolations, indexes := groupViolationsByBeadIndex(violations)
	for _, beadIndex := range indexes {
		sb.WriteString(fmt.Sprintf("- Bead %d:\n", beadIndex))
		for _, v := range groupedViolations[beadIndex] {
			sb.WriteString(fmt.Sprintf("  - [%s] %s\n", v.Rule, v.Message))
		}
	}
	sb.WriteString("\n")

	sb.WriteString(repromptInstructionsHeader)
	sb.WriteString(repromptInstructionsBody)

	return sb.String()
}

func groupViolationsByBeadIndex(violations []Violation) (map[int][]Violation, []int) {
	groupedViolations := make(map[int][]Violation)
	for _, violation := range violations {
		groupedViolations[violation.BeadIndex] = append(groupedViolations[violation.BeadIndex], violation)
	}

	indexes := make([]int, 0, len(groupedViolations))
	for beadIndex := range groupedViolations {
		indexes = append(indexes, beadIndex)
	}
	sort.Ints(indexes)

	return groupedViolations, indexes
}
