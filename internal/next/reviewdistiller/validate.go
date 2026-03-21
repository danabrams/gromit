package reviewdistiller

import (
	"fmt"
)

// ValidateProposals validates that proposals conform to outcome-specific requirements.
// Returns an error if:
// - The outcome type is not recognized (must be accepted, rework_implementation_gap, or rework_vision_change)
// - The proposals do not contain the required types for the given outcome
//
// Validation rules:
// - accepted: at least one proposal must have type doctrine_rule or planner_heuristic
// - rework_implementation_gap: at least one proposal must have type validation_gap, doctrine_rule, or planner_heuristic
// - rework_vision_change: at least one proposal must have type refinement_guidance
func ValidateProposals(outcome string, proposals []Proposal) error {
	// Validate outcome type is recognized
	switch outcome {
	case "accepted":
		return validateAccepted(proposals)
	case "rework_implementation_gap":
		return validateReworkImplementationGap(proposals)
	case "rework_vision_change":
		return validateReworkVisionChange(proposals)
	default:
		return fmt.Errorf("unrecognized outcome type: %q (must be one of: accepted, rework_implementation_gap, rework_vision_change)", outcome)
	}
}

// validateAccepted validates that at least one proposal is doctrine_rule or planner_heuristic.
func validateAccepted(proposals []Proposal) error {
	for _, p := range proposals {
		if p.Type == "doctrine_rule" || p.Type == "planner_heuristic" {
			return nil
		}
	}
	return fmt.Errorf("outcome accepted requires at least one proposal of type doctrine_rule or planner_heuristic")
}

// validateReworkImplementationGap validates that at least one proposal is validation_gap, doctrine_rule, or planner_heuristic.
func validateReworkImplementationGap(proposals []Proposal) error {
	for _, p := range proposals {
		if p.Type == "validation_gap" || p.Type == "doctrine_rule" || p.Type == "planner_heuristic" {
			return nil
		}
	}
	return fmt.Errorf("outcome rework_implementation_gap requires at least one proposal of type validation_gap, doctrine_rule, or planner_heuristic")
}

// validateReworkVisionChange validates that at least one proposal is refinement_guidance.
func validateReworkVisionChange(proposals []Proposal) error {
	for _, p := range proposals {
		if p.Type == "refinement_guidance" {
			return nil
		}
	}
	return fmt.Errorf("outcome rework_vision_change requires at least one proposal of type refinement_guidance")
}
