package reviewdistiller

import (
	"testing"
)

// TestValidateProposalsAcceptedWithDoctrineRule validates accepted outcome with doctrine_rule.
func TestValidateProposalsAcceptedWithDoctrineRule(t *testing.T) {
	proposals := []Proposal{
		{ID: "p1", Type: "doctrine_rule", Title: "Add validation check"},
	}
	err := ValidateProposals("accepted", proposals)
	if err != nil {
		t.Errorf("ValidateProposals failed with acceptable doctrine_rule: %v", err)
	}
}

// TestValidateProposalsAcceptedWithPlannerHeuristic validates accepted outcome with planner_heuristic.
func TestValidateProposalsAcceptedWithPlannerHeuristic(t *testing.T) {
	proposals := []Proposal{
		{ID: "p1", Type: "planner_heuristic", Title: "Improve planning logic"},
	}
	err := ValidateProposals("accepted", proposals)
	if err != nil {
		t.Errorf("ValidateProposals failed with acceptable planner_heuristic: %v", err)
	}
}

// TestValidateProposalsAcceptedWithBoth validates accepted outcome with both types.
func TestValidateProposalsAcceptedWithBoth(t *testing.T) {
	proposals := []Proposal{
		{ID: "p1", Type: "doctrine_rule", Title: "Add rule"},
		{ID: "p2", Type: "planner_heuristic", Title: "Improve heuristic"},
		{ID: "p3", Type: "refinement_guidance", Title: "Refine docs"},
	}
	err := ValidateProposals("accepted", proposals)
	if err != nil {
		t.Errorf("ValidateProposals failed with multiple acceptable types: %v", err)
	}
}

// TestValidateProposalsAcceptedRejects validates that accepted rejects non-conforming proposals.
func TestValidateProposalsAcceptedRejects(t *testing.T) {
	proposals := []Proposal{
		{ID: "p1", Type: "validation_gap", Title: "Add validation"},
		{ID: "p2", Type: "refinement_guidance", Title: "Refine docs"},
	}
	err := ValidateProposals("accepted", proposals)
	if err == nil {
		t.Error("ValidateProposals should reject accepted outcome without doctrine_rule or planner_heuristic")
	}
}

// TestValidateProposalsReworkImplementationGapWithValidationGap validates rework_implementation_gap with validation_gap.
func TestValidateProposalsReworkImplementationGapWithValidationGap(t *testing.T) {
	proposals := []Proposal{
		{ID: "p1", Type: "validation_gap", Title: "Missing test"},
	}
	err := ValidateProposals("rework_implementation_gap", proposals)
	if err != nil {
		t.Errorf("ValidateProposals failed with acceptable validation_gap: %v", err)
	}
}

// TestValidateProposalsReworkImplementationGapWithDoctrineRule validates rework_implementation_gap with doctrine_rule.
func TestValidateProposalsReworkImplementationGapWithDoctrineRule(t *testing.T) {
	proposals := []Proposal{
		{ID: "p1", Type: "doctrine_rule", Title: "Add rule"},
	}
	err := ValidateProposals("rework_implementation_gap", proposals)
	if err != nil {
		t.Errorf("ValidateProposals failed with acceptable doctrine_rule: %v", err)
	}
}

// TestValidateProposalsReworkImplementationGapWithPlannerHeuristic validates rework_implementation_gap with planner_heuristic.
func TestValidateProposalsReworkImplementationGapWithPlannerHeuristic(t *testing.T) {
	proposals := []Proposal{
		{ID: "p1", Type: "planner_heuristic", Title: "Improve planning"},
	}
	err := ValidateProposals("rework_implementation_gap", proposals)
	if err != nil {
		t.Errorf("ValidateProposals failed with acceptable planner_heuristic: %v", err)
	}
}

// TestValidateProposalsReworkImplementationGapRejects validates that rework_implementation_gap rejects non-conforming proposals.
func TestValidateProposalsReworkImplementationGapRejects(t *testing.T) {
	proposals := []Proposal{
		{ID: "p1", Type: "refinement_guidance", Title: "Refine docs"},
	}
	err := ValidateProposals("rework_implementation_gap", proposals)
	if err == nil {
		t.Error("ValidateProposals should reject rework_implementation_gap without validation_gap, doctrine_rule, or planner_heuristic")
	}
}

// TestValidateProposalsReworkVisionChangeWithRefinementGuidance validates rework_vision_change with refinement_guidance.
func TestValidateProposalsReworkVisionChangeWithRefinementGuidance(t *testing.T) {
	proposals := []Proposal{
		{ID: "p1", Type: "refinement_guidance", Title: "Clarify spec"},
	}
	err := ValidateProposals("rework_vision_change", proposals)
	if err != nil {
		t.Errorf("ValidateProposals failed with acceptable refinement_guidance: %v", err)
	}
}

// TestValidateProposalsReworkVisionChangeRejects validates that rework_vision_change rejects non-conforming proposals.
func TestValidateProposalsReworkVisionChangeRejects(t *testing.T) {
	proposals := []Proposal{
		{ID: "p1", Type: "validation_gap", Title: "Add validation"},
		{ID: "p2", Type: "doctrine_rule", Title: "Add rule"},
	}
	err := ValidateProposals("rework_vision_change", proposals)
	if err == nil {
		t.Error("ValidateProposals should reject rework_vision_change without refinement_guidance")
	}
}

// TestValidateProposalsUnrecognizedOutcome validates that unrecognized outcomes are rejected.
func TestValidateProposalsUnrecognizedOutcome(t *testing.T) {
	proposals := []Proposal{
		{ID: "p1", Type: "doctrine_rule", Title: "Add rule"},
	}
	err := ValidateProposals("rejected", proposals)
	if err == nil {
		t.Error("ValidateProposals should reject unrecognized outcome type")
	}
}

// TestValidateProposalsEmptyProposals validates that empty proposals fail all validations.
func TestValidateProposalsEmptyProposals(t *testing.T) {
	proposals := []Proposal{}

	tests := []string{"accepted", "rework_implementation_gap", "rework_vision_change"}
	for _, outcome := range tests {
		err := ValidateProposals(outcome, proposals)
		if err == nil {
			t.Errorf("ValidateProposals should reject empty proposals for outcome %q", outcome)
		}
	}
}

// TestValidateProposalsNilProposals validates that nil proposals fail all validations.
func TestValidateProposalsNilProposals(t *testing.T) {
	var proposals []Proposal

	tests := []string{"accepted", "rework_implementation_gap", "rework_vision_change"}
	for _, outcome := range tests {
		err := ValidateProposals(outcome, proposals)
		if err == nil {
			t.Errorf("ValidateProposals should reject nil proposals for outcome %q", outcome)
		}
	}
}
