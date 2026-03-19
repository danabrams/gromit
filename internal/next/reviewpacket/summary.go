package reviewpacket

import (
	"fmt"
)

// GenerateSummaryInput contains the data needed to generate a review summary.
type GenerateSummaryInput struct {
	TerminalState    string
	BehaviorCardLen  int
	BehaviorStatus   map[string]int // status -> count, e.g. {"proven": 5, "mixed": 1}
	AcceptancePassed int
	AcceptanceTotal  int
	RepairCycles     int
	DegradedFlags    []string
}

// GenerateSummary produces a deterministic, template-based summary string.
// For ready_for_review runs, it describes the overall outcome.
// For diagnostic runs (blocked, needs_human), it summarizes the blocker situation.
func GenerateSummary(input GenerateSummaryInput) string {
	if input.TerminalState == "ready_for_review" {
		return generateReadyForReviewSummary(input)
	}
	return generateDiagnosticSummary(input)
}

func generateReadyForReviewSummary(input GenerateSummaryInput) string {
	// Determine the dominant behavior status
	var dominantStatus string
	var dominantCount int
	for status, count := range input.BehaviorStatus {
		if count > dominantCount {
			dominantCount = count
			dominantStatus = status
		}
	}

	// Build behavior part
	behaviorPart := fmt.Sprintf("%d behaviors verified", input.BehaviorCardLen)
	if dominantCount > 0 {
		if dominantCount == input.BehaviorCardLen {
			// All behaviors have same status
			behaviorPart += fmt.Sprintf(", all %s", dominantStatus)
		} else {
			// Mixed statuses
			behaviorPart += fmt.Sprintf(", mostly %s", dominantStatus)
		}
	}

	// Build acceptance part
	acceptancePart := fmt.Sprintf("%d/%d acceptance criteria passed", input.AcceptancePassed, input.AcceptanceTotal)

	return fmt.Sprintf("%s. %s", behaviorPart, acceptancePart)
}

func generateDiagnosticSummary(input GenerateSummaryInput) string {
	var reason string
	switch input.TerminalState {
	case "blocked":
		reason = "Run blocked"
	case "needs_human":
		reason = "Run needs human review"
	default:
		reason = fmt.Sprintf("Run ended in %s", input.TerminalState)
	}

	if input.RepairCycles > 0 {
		reason += fmt.Sprintf(": validation failed after %d repair cycle", input.RepairCycles)
		if input.RepairCycles > 1 {
			reason += "s"
		}
	}

	return reason
}
