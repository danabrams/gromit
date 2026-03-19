package reviewpacket

import (
	"fmt"
)

// DetectSurprises identifies unexpected conditions in the run evidence.
// It returns a list of surprise descriptions that should be included in the ProductReview.
//
// Surprises detected:
// - All acceptance criteria passed despite degraded evidence (degraded flags exist)
// - Mismatch between scenario count and acceptance pass count
// - Blocked or needs_human run with all acceptance criteria passed
func DetectSurprises(inputs Inputs) []string {
	var surprises []string

	// Parse scenarios from spec content
	scenarios := ParseScenarios(inputs.SpecContent)

	// Read acceptance result data directly from concrete type
	passedCount := inputs.AcceptanceResult.Passed
	failedCount := inputs.AcceptanceResult.Failed
	unclearCount := inputs.AcceptanceResult.Unclear

	totalAcceptance := passedCount + failedCount + unclearCount
	allAcceptancePassed := totalAcceptance > 0 && failedCount == 0 && unclearCount == 0

	// Surprise 1: Acceptance passed despite degraded evidence
	if allAcceptancePassed && len(inputs.DegradedFlags) > 0 {
		surprises = append(surprises,
			fmt.Sprintf("All acceptance criteria passed despite degraded evidence: %v", inputs.DegradedFlags))
	}

	// Surprise 2: Scenario count mismatch (more scenarios than passed criteria is surprising)
	scenarioCount := len(scenarios)
	if scenarioCount > 0 && scenarioCount > passedCount {
		surprises = append(surprises,
			fmt.Sprintf("Scenario count mismatch: %d scenarios in spec but only %d acceptance criteria passed",
				scenarioCount, passedCount))
	}

	// Surprise 3: Blocked/needs_human run with passed acceptance
	if (inputs.TerminalState == "blocked" || inputs.TerminalState == "needs_human") && allAcceptancePassed {
		surprises = append(surprises,
			fmt.Sprintf("Run is %s but all acceptance criteria passed", inputs.TerminalState))
	}

	return surprises
}
