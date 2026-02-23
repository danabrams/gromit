package validate

import "fmt"

// ComplexityResult describes the complexity classification for one candidate.
type ComplexityResult struct {
	Title          string
	Classification string
	Reasons        []string
}

// ComplexityOutcome summarizes high-complexity candidates across a batch.
type ComplexityOutcome struct {
	HighCount        int
	HighComplexity   []CandidateComplexityResult
	AggregateReasons []string
}

// ScoreCandidate classifies one candidate's complexity and records scoring reasons.
func ScoreCandidate(bead BeadCandidate) ComplexityResult {
	result := ComplexityResult{
		Title:          bead.Title,
		Classification: "low",
	}

	if bead.EstimatedFiles > highComplexityFileThreshold {
		result.Reasons = append(result.Reasons, fmt.Sprintf("estimated_files=%d crosses the high-complexity threshold", bead.EstimatedFiles))
	}
	if len(bead.AcceptanceCriteria) >= 3 {
		result.Reasons = append(result.Reasons, fmt.Sprintf("acceptance_criteria=%d indicates broad implementation surface", len(bead.AcceptanceCriteria)))
	}
	if len(bead.ExpectedOutputs) >= 4 {
		result.Reasons = append(result.Reasons, fmt.Sprintf("expected_outputs=%d indicates multiple coupled deliverables", len(bead.ExpectedOutputs)))
	}
	if containsDependencySignal(bead.Title) || containsDependencySignal(bead.Description) {
		result.Reasons = append(result.Reasons, "contains explicit dependency sequencing language")
	}
	titleHasScopeSignal := containsScopeSignal(bead.Title)
	descriptionHasScopeSignal := containsScopeSignal(bead.Description)
	if titleHasScopeSignal {
		result.Reasons = append(result.Reasons, "contains broad-scope language in title")
	}
	if descriptionHasScopeSignal {
		result.Reasons = append(result.Reasons, "contains broad-scope language in description")
	}
	if len(result.Reasons) > 0 {
		result.Classification = "high"
	} else {
		result.Reasons = append(result.Reasons, "no high-complexity signals detected")
	}

	return result
}
