package reviewpacket

import (
	"fmt"
)

// Generator builds review packet artifacts from run evidence.
type Generator struct{}

// Generate assembles ProductReview, ProcessReview, and ManualChecklist from Inputs.
// It handles both ready_for_review and diagnostic (blocked/needs_human) variants.
func (g *Generator) Generate(inputs Inputs) (Outputs, error) {
	// Parse scenarios from spec content
	scenarios := ParseScenarios(inputs.SpecContent)

	// If no scenarios, fall back to acceptance criteria
	var behaviorSources interface{}
	if len(scenarios) == 0 {
		criteria := ParseAcceptanceCriteria(inputs.SpecContent)
		behaviorSources = criteria
	} else {
		behaviorSources = scenarios
	}

	// Read validation result data
	validationPassed := inputs.ValidationResult.Passed
	validationChecks := inputs.ValidationResult.Checks

	// Read acceptance result data
	acceptancePassed := inputs.AcceptanceResult.Passed
	acceptanceFailed := inputs.AcceptanceResult.Failed
	acceptanceUnclear := inputs.AcceptanceResult.Unclear
	acceptanceTotal := acceptancePassed + acceptanceFailed + acceptanceUnclear
	acceptanceAllPassed := acceptanceTotal > 0 && acceptanceFailed == 0 && acceptanceUnclear == 0

	// Determine if there are blocking findings
	blockingFindings, ok := inputs.ReviewFindings["blocking"]
	hasBlockingFindings := ok && len(blockingFindings) > 0

	// Determine behavior card status based on run evidence
	cardStatus := determineBehaviorCardStatus(
		inputs.TerminalState,
		validationPassed,
		acceptanceAllPassed,
		hasBlockingFindings,
		len(inputs.DegradedFlags) > 0,
		acceptanceTotal,
	)

	// Generate behavior cards from scenarios or criteria
	behaviorCards := generateBehaviorCards(behaviorSources, cardStatus)

	// Build ProductReview
	productReview := ProductReview{
		RunID:         inputs.RunID,
		SpecTitle:     inputs.SpecTitle,
		TerminalState: inputs.TerminalState,
		BehaviorCards: behaviorCards,
		IsDiagnostic:  inputs.TerminalState != "ready_for_review",
	}

	// Compute trust level
	trustLevel := ComputeTrustLevel(
		inputs.TerminalState,
		validationPassed,
		acceptanceAllPassed,
		hasBlockingFindings,
		len(inputs.DegradedFlags) > 0,
		inputs.RepeatedFailure,
	)

	// Generate summary
	behaviorStatusCounts := countBehaviorStatuses(behaviorCards)
	summaryInput := GenerateSummaryInput{
		TerminalState:    inputs.TerminalState,
		BehaviorCardLen:  len(behaviorCards),
		BehaviorStatus:   behaviorStatusCounts,
		AcceptancePassed: acceptancePassed,
		AcceptanceTotal:  acceptanceTotal,
		RepairCycles:     inputs.RepairCycles,
		DegradedFlags:    inputs.DegradedFlags,
	}
	productReview.Summary = GenerateSummary(summaryInput)

	// For diagnostic variants, populate blocker/next action and set card status to unclear
	if productReview.IsDiagnostic {
		productReview.BlockerSummary = generateBlockerSummary(inputs)
		productReview.RecommendedNextAction = generateRecommendedAction(inputs)
		for i := range productReview.BehaviorCards {
			productReview.BehaviorCards[i].AutomaticStatus = "unclear"
		}
	}

	// Detect surprises
	productReview.Surprises = DetectSurprises(inputs)

	// Build ProcessReview
	processReview := ProcessReview{
		TrustLevel:          trustLevel,
		RecommendedPosture:  RecommendedPosture(trustLevel),
		AutomaticProof:      formatValidationSummary(validationPassed, validationChecks),
		MachineReview:       formatReviewSummary(inputs.ReviewFindings),
		Acceptance:          formatAcceptanceSummary(acceptancePassed, acceptanceTotal),
		DegradedFlags:       inputs.DegradedFlags,
		RepairCycles:        inputs.RepairCycles,
		RepeatedFailureFlag: inputs.RepeatedFailure,
	}

	// Build ManualChecklist
	var manualChecklist ManualChecklist
	if productReview.IsDiagnostic {
		// Empty items for diagnostic runs
		manualChecklist = ManualChecklist{Items: []ManualCheckItem{}}
	} else {
		// Parse manual checks (explicit or derived from scenarios)
		manualChecklist = ParseManualChecks(inputs.SpecContent, scenarios)
	}

	// Normalize all nil fields
	productReview.NormalizeNilFields()
	processReview.NormalizeNilFields()
	manualChecklist.NormalizeNilFields()

	// Normalize behavior cards
	for i := range productReview.BehaviorCards {
		productReview.BehaviorCards[i].NormalizeNilFields()
	}

	// Normalize manual check items
	for i := range manualChecklist.Items {
		manualChecklist.Items[i].NormalizeNilFields()
	}

	return Outputs{
		ProductReview:   productReview,
		ProcessReview:   processReview,
		ManualChecklist: manualChecklist,
	}, nil
}

// determineBehaviorCardStatus determines the automatic_status for behavior cards.
func determineBehaviorCardStatus(terminalState string, validationPassed bool, acceptanceAllPassed bool,
	hasBlockingFindings bool, hasDegradedFlags bool, acceptanceTotal int) string {
	if terminalState != "ready_for_review" {
		return "unclear"
	}

	// proven: validation passed AND all acceptance passed AND no blocking findings AND no degraded flags
	if validationPassed && acceptanceAllPassed && !hasBlockingFindings && !hasDegradedFlags {
		return "proven"
	}

	// unclear: no acceptance data at all — insufficient evidence to judge
	if acceptanceTotal == 0 && !hasBlockingFindings {
		if validationPassed {
			return "mixed"
		}
		return "unclear"
	}

	// failed: acceptance failed OR blocking findings exist
	if !acceptanceAllPassed || hasBlockingFindings {
		return "failed"
	}

	// mixed: validation passed but some concerns exist
	if validationPassed {
		return "mixed"
	}

	// unclear: insufficient evidence
	return "unclear"
}

// generateBehaviorCards creates behavior cards from scenarios or criteria.
func generateBehaviorCards(source interface{}, status string) []BehaviorCard {
	var cards []BehaviorCard

	switch v := source.(type) {
	case []ParsedScenario:
		for i, scenario := range v {
			card := BehaviorCard{
				ID:              fmt.Sprintf("behavior_%d", i+1),
				Title:           scenario.Title,
				Given:           scenario.Given,
				When:            scenario.When,
				Then:            scenario.Then,
				AutomaticStatus: status,
				EvidenceFiles:   []string{"validation.json", "acceptance.json", "review.json"},
			}
			cards = append(cards, card)
		}
	case []ParsedCriterion:
		for i, criterion := range v {
			card := BehaviorCard{
				ID:              fmt.Sprintf("behavior_%d", i+1),
				Title:           criterion.Text,
				AutomaticStatus: status,
				EvidenceFiles:   []string{"validation.json", "acceptance.json", "review.json"},
			}
			cards = append(cards, card)
		}
	}

	return cards
}

// countBehaviorStatuses counts behavior cards by their status.
func countBehaviorStatuses(cards []BehaviorCard) map[string]int {
	counts := make(map[string]int)
	for _, card := range cards {
		counts[card.AutomaticStatus]++
	}
	return counts
}

// generateBlockerSummary creates a blocker summary for diagnostic runs.
func generateBlockerSummary(inputs Inputs) string {
	if inputs.TerminalState == "blocked" {
		return "Run is blocked - validation failed and requires investigation"
	}
	if inputs.TerminalState == "needs_human" {
		return "Run requires human review - automatic validation did not complete successfully"
	}
	return ""
}

// generateRecommendedAction creates a recommended next action for diagnostic runs.
func generateRecommendedAction(inputs Inputs) string {
	if inputs.TerminalState == "blocked" {
		return "Review validation failures and repair implementation before retrying"
	}
	if inputs.TerminalState == "needs_human" {
		return "Perform manual review and determine if changes are needed"
	}
	return ""
}

// formatValidationSummary formats a validation summary string.
func formatValidationSummary(passed bool, checks int) string {
	if passed {
		return fmt.Sprintf("all %d validation checks passed", checks)
	}
	return fmt.Sprintf("validation incomplete or failed (%d checks attempted)", checks)
}

// formatReviewSummary formats a review findings summary.
func formatReviewSummary(findings map[string][]ReviewFinding) string {
	if len(findings) == 0 {
		return "no findings"
	}

	blockingCount := len(findings["blocking"])
	totalCount := 0
	for _, issues := range findings {
		totalCount += len(issues)
	}

	if blockingCount > 0 {
		return fmt.Sprintf("%d findings (%d blocking)", totalCount, blockingCount)
	}
	return fmt.Sprintf("%d findings (0 blocking)", totalCount)
}

// formatAcceptanceSummary formats an acceptance summary string.
func formatAcceptanceSummary(passed int, total int) string {
	return fmt.Sprintf("%d/%d criteria passed", passed, total)
}
