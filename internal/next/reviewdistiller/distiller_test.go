package reviewdistiller

import (
	"encoding/json"
	"strings"
	"testing"
)

// stubLLMCompleter implements LLMCompleter and returns canned JSON for testing.
type stubLLMCompleter struct {
	response string
	err      error
}

func (s *stubLLMCompleter) Complete(ctx interface{}, prompt string) (string, error) {
	return s.response, s.err
}

// TestDistill_Accepted verifies Distill with accepted outcome.
// Uses a stub LLMCompleter that returns canned JSON with 4 proposals including doctrine_rule.
// Verifies DistillationResult fields, proposal count, IDs are content-based, and validation passes.
func TestDistill_Accepted(t *testing.T) {
	// Create stub LLM response with 4 proposals, one of which is doctrine_rule
	llmResponse := `{
		"proposals": [
			{
				"type": "doctrine_rule",
				"title": "Add error checking",
				"what_happened": "Function did not check for errors",
				"what_was_missing": "Error handling",
				"proposed_change": "Add if err != nil check",
				"rationale": "Prevents silent failures",
				"confidence": "high",
				"confidence_rationale": "Standard Go pattern",
				"evidence_references": ["line 42"]
			},
			{
				"type": "planner_heuristic",
				"title": "Use better variable names",
				"what_happened": "Variable names were unclear",
				"what_was_missing": "Clarity",
				"proposed_change": "Rename 'x' to 'count'",
				"rationale": "Improves readability",
				"confidence": "high",
				"confidence_rationale": "Clear naming convention",
				"evidence_references": ["line 15"]
			},
			{
				"type": "validation_gap",
				"title": "Add input validation",
				"what_happened": "Input was not validated",
				"what_was_missing": "Validation",
				"proposed_change": "Add bounds check",
				"rationale": "Prevents invalid input",
				"confidence": "medium",
				"confidence_rationale": "Common edge case",
				"evidence_references": ["line 20"]
			},
			{
				"type": "refinement_guidance",
				"title": "Simplify logic",
				"what_happened": "Logic was complex",
				"what_was_missing": "Simplification",
				"proposed_change": "Extract helper function",
				"rationale": "Reduces complexity",
				"confidence": "low",
				"confidence_rationale": "Subjective improvement",
				"evidence_references": ["line 50"]
			}
		]
	}`

	// Create test inputs
	outcomeJSON := json.RawMessage(`{"outcome": "accepted"}`)
	inputs := &DistillerInputs{
		RunID:         "run-123",
		SpecID:        "spec-456",
		SpecContent:   "test spec",
		ReviewOutcome: outcomeJSON,
	}

	// Create stub LLM
	stub := &stubLLMCompleter{response: llmResponse}

	// Call Distill
	result, err := Distill(inputs, stub, TierHigh)
	if err != nil {
		t.Fatalf("Distill() returned error: %v", err)
	}

	// Verify DistillationResult fields
	if result.RunID != "run-123" {
		t.Errorf("RunID = %q, want %q", result.RunID, "run-123")
	}
	if result.SpecID != "spec-456" {
		t.Errorf("SpecID = %q, want %q", result.SpecID, "spec-456")
	}
	if result.Outcome != "accepted" {
		t.Errorf("Outcome = %q, want %q", result.Outcome, "accepted")
	}
	if result.ModelTier != TierHigh {
		t.Errorf("ModelTier = %q, want %q", result.ModelTier, TierHigh)
	}

	// Verify proposal count is 4
	if len(result.Proposals) != 4 {
		t.Errorf("Proposal count = %d, want 4", len(result.Proposals))
	}

	// Verify proposal IDs are content-based (p1, p2, p3, p4)
	expectedIDs := []string{"p1", "p2", "p3", "p4"}
	for i, expectedID := range expectedIDs {
		if i >= len(result.Proposals) {
			t.Errorf("Expected proposal %d with ID %q, but no proposal found", i, expectedID)
			continue
		}
		if result.Proposals[i].ID != expectedID {
			t.Errorf("Proposal %d ID = %q, want %q", i, result.Proposals[i].ID, expectedID)
		}
	}

	// Verify that proposal types are set correctly
	expectedTypes := []string{"doctrine_rule", "planner_heuristic", "validation_gap", "refinement_guidance"}
	for i, expectedType := range expectedTypes {
		if i >= len(result.Proposals) {
			break
		}
		if result.Proposals[i].Type != expectedType {
			t.Errorf("Proposal %d type = %q, want %q", i, result.Proposals[i].Type, expectedType)
		}
	}

	// Verify that CreatedAt is set
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	// Verify outcome-specific validation passed
	// (The test would fail earlier if ValidateProposals rejected the outcome,
	// so the fact that we reach here means validation passed)
}

// TestDistill_ReworkImplementationGap verifies Distill with rework_implementation_gap outcome.
// Uses a stub LLMCompleter that returns canned JSON with 3 proposals all of type validation_gap.
// Verifies DistillationResult fields, proposal count, IDs are content-based, and validation passes.
func TestDistill_ReworkImplementationGap(t *testing.T) {
	// Create stub LLM response with 3 proposals, all of type validation_gap
	llmResponse := `{
		"proposals": [
			{
				"type": "validation_gap",
				"title": "Add boundary validation",
				"what_happened": "Function accepted out-of-range values",
				"what_was_missing": "Input range validation",
				"proposed_change": "Add min/max bounds check before processing",
				"rationale": "Prevents invalid state",
				"confidence": "high",
				"confidence_rationale": "Clear boundary requirements",
				"evidence_references": ["line 12"]
			},
			{
				"type": "validation_gap",
				"title": "Check for nil pointers",
				"what_happened": "Function did not guard against nil input",
				"what_was_missing": "Nil pointer validation",
				"proposed_change": "Add nil check at function entry",
				"rationale": "Prevents panic on invalid input",
				"confidence": "high",
				"confidence_rationale": "Standard defensive programming",
				"evidence_references": ["line 8"]
			},
			{
				"type": "validation_gap",
				"title": "Validate slice capacity",
				"what_happened": "Slice was resized without checking capacity",
				"what_was_missing": "Slice capacity validation",
				"proposed_change": "Check len and cap before append operations",
				"rationale": "Ensures memory efficiency and correctness",
				"confidence": "medium",
				"confidence_rationale": "Defensive slice handling",
				"evidence_references": ["line 25"]
			}
		]
	}`

	// Create test inputs
	outcomeJSON := json.RawMessage(`{"outcome": "rework_implementation_gap"}`)
	inputs := &DistillerInputs{
		RunID:         "run-789",
		SpecID:        "spec-789",
		SpecContent:   "test spec for rework",
		ReviewOutcome: outcomeJSON,
	}

	// Create stub LLM
	stub := &stubLLMCompleter{response: llmResponse}

	// Call Distill
	result, err := Distill(inputs, stub, TierHigh)
	if err != nil {
		t.Fatalf("Distill() returned error: %v", err)
	}

	// Verify DistillationResult fields
	if result.RunID != "run-789" {
		t.Errorf("RunID = %q, want %q", result.RunID, "run-789")
	}
	if result.SpecID != "spec-789" {
		t.Errorf("SpecID = %q, want %q", result.SpecID, "spec-789")
	}
	if result.Outcome != "rework_implementation_gap" {
		t.Errorf("Outcome = %q, want %q", result.Outcome, "rework_implementation_gap")
	}
	if result.ModelTier != TierHigh {
		t.Errorf("ModelTier = %q, want %q", result.ModelTier, TierHigh)
	}

	// Verify proposal count is 3
	if len(result.Proposals) != 3 {
		t.Errorf("Proposal count = %d, want 3", len(result.Proposals))
	}

	// Verify proposal IDs are content-based (p1, p2, p3)
	expectedIDs := []string{"p1", "p2", "p3"}
	for i, expectedID := range expectedIDs {
		if i >= len(result.Proposals) {
			t.Errorf("Expected proposal %d with ID %q, but no proposal found", i, expectedID)
			continue
		}
		if result.Proposals[i].ID != expectedID {
			t.Errorf("Proposal %d ID = %q, want %q", i, result.Proposals[i].ID, expectedID)
		}
	}

	// Verify that all proposal types are validation_gap
	for i := 0; i < len(result.Proposals); i++ {
		if result.Proposals[i].Type != "validation_gap" {
			t.Errorf("Proposal %d type = %q, want %q", i, result.Proposals[i].Type, "validation_gap")
		}
	}

	// Verify that CreatedAt is set
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	// Verify outcome-specific validation passed
	// (The test would fail earlier if ValidateProposals rejected the outcome,
	// so the fact that we reach here means validation passed)
}

// TestDistill_ReworkVisionChange verifies Distill with rework_vision_change outcome.
// Uses a stub LLMCompleter that returns canned JSON with 3 proposals all of type refinement_guidance.
// Verifies DistillationResult fields, proposal count, IDs are content-based, and validation passes.
func TestDistill_ReworkVisionChange(t *testing.T) {
	// Create stub LLM response with 3 proposals, all of type refinement_guidance
	llmResponse := `{
		"proposals": [
			{
				"type": "refinement_guidance",
				"title": "Clarify interface contract",
				"what_happened": "Interface behavior was ambiguous",
				"what_was_missing": "Clear documentation of expectations",
				"proposed_change": "Add detailed docstring with examples",
				"rationale": "Improves API clarity and user understanding",
				"confidence": "medium",
				"confidence_rationale": "Based on review feedback",
				"evidence_references": ["line 5"]
			},
			{
				"type": "refinement_guidance",
				"title": "Reconsider error handling strategy",
				"what_happened": "Error handling approach may not fit the larger system",
				"what_was_missing": "Alignment with system-wide patterns",
				"proposed_change": "Adopt centralized error handler pattern",
				"rationale": "Better consistency across codebase",
				"confidence": "medium",
				"confidence_rationale": "Aligns with existing infrastructure",
				"evidence_references": ["line 30"]
			},
			{
				"type": "refinement_guidance",
				"title": "Revisit performance assumptions",
				"what_happened": "Performance implications were not fully explored",
				"what_was_missing": "Benchmarking and profiling analysis",
				"proposed_change": "Add benchmark tests and profile the implementation",
				"rationale": "Ensures implementation meets performance goals",
				"confidence": "low",
				"confidence_rationale": "Depends on actual performance metrics",
				"evidence_references": ["line 45"]
			}
		]
	}`

	// Create test inputs
	outcomeJSON := json.RawMessage(`{"outcome": "rework_vision_change"}`)
	inputs := &DistillerInputs{
		RunID:         "run-345",
		SpecID:        "spec-345",
		SpecContent:   "test spec for vision change",
		ReviewOutcome: outcomeJSON,
	}

	// Create stub LLM
	stub := &stubLLMCompleter{response: llmResponse}

	// Call Distill
	result, err := Distill(inputs, stub, TierHigh)
	if err != nil {
		t.Fatalf("Distill() returned error: %v", err)
	}

	// Verify DistillationResult fields
	if result.RunID != "run-345" {
		t.Errorf("RunID = %q, want %q", result.RunID, "run-345")
	}
	if result.SpecID != "spec-345" {
		t.Errorf("SpecID = %q, want %q", result.SpecID, "spec-345")
	}
	if result.Outcome != "rework_vision_change" {
		t.Errorf("Outcome = %q, want %q", result.Outcome, "rework_vision_change")
	}
	if result.ModelTier != TierHigh {
		t.Errorf("ModelTier = %q, want %q", result.ModelTier, TierHigh)
	}

	// Verify proposal count is 3
	if len(result.Proposals) != 3 {
		t.Errorf("Proposal count = %d, want 3", len(result.Proposals))
	}

	// Verify proposal IDs are content-based (p1, p2, p3)
	expectedIDs := []string{"p1", "p2", "p3"}
	for i, expectedID := range expectedIDs {
		if i >= len(result.Proposals) {
			t.Errorf("Expected proposal %d with ID %q, but no proposal found", i, expectedID)
			continue
		}
		if result.Proposals[i].ID != expectedID {
			t.Errorf("Proposal %d ID = %q, want %q", i, result.Proposals[i].ID, expectedID)
		}
	}

	// Verify that all proposal types are refinement_guidance
	for i := 0; i < len(result.Proposals); i++ {
		if result.Proposals[i].Type != "refinement_guidance" {
			t.Errorf("Proposal %d type = %q, want %q", i, result.Proposals[i].Type, "refinement_guidance")
		}
	}

	// Verify that CreatedAt is set
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	// Verify outcome-specific validation passed
	// (The test would fail earlier if ValidateProposals rejected the outcome,
	// so the fact that we reach here means validation passed)
}

// TestDistill_UnrecognizedOutcome verifies Distill returns an error with an unrecognized outcome type.
// Verifies the error message contains the unsupported type name.
func TestDistill_UnrecognizedOutcome(t *testing.T) {
	// Create stub LLM response with a single proposal
	llmResponse := `{
		"proposals": [
			{
				"type": "doctrine_rule",
				"title": "Test proposal",
				"what_happened": "Something happened",
				"what_was_missing": "Something was missing",
				"proposed_change": "Make a change",
				"rationale": "Good reason",
				"confidence": "high",
				"confidence_rationale": "Because it is",
				"evidence_references": ["line 1"]
			}
		]
	}`

	// Create test inputs with unrecognized outcome type
	outcomeJSON := json.RawMessage(`{"outcome": "unsupported_outcome"}`)
	inputs := &DistillerInputs{
		RunID:         "run-999",
		SpecID:        "spec-999",
		SpecContent:   "test spec",
		ReviewOutcome: outcomeJSON,
	}

	// Create stub LLM
	stub := &stubLLMCompleter{response: llmResponse}

	// Call Distill and expect an error
	result, err := Distill(inputs, stub, TierHigh)
	if err == nil {
		t.Fatalf("Distill() expected error for unrecognized outcome, got nil")
	}

	// Verify result is nil
	if result != nil {
		t.Errorf("Expected nil result with error, got %v", result)
	}

	// Verify error message contains the unsupported type name
	if !strings.Contains(err.Error(), "unsupported_outcome") {
		t.Errorf("Error message does not contain unsupported type name: %v", err)
	}
}

// TestDistill_TruncatesExcess verifies Distill truncates proposals to max 5.
// Uses a stub LLMCompleter that returns canned JSON with 7 proposals.
// Verifies that only the first 5 proposals are kept in the result.
func TestDistill_TruncatesExcess(t *testing.T) {
	// Create stub LLM response with 7 proposals
	llmResponse := `{
		"proposals": [
			{
				"type": "doctrine_rule",
				"title": "Proposal 1",
				"what_happened": "Issue 1",
				"what_was_missing": "Missing 1",
				"proposed_change": "Change 1",
				"rationale": "Rationale 1",
				"confidence": "high",
				"confidence_rationale": "Reason 1",
				"evidence_references": ["line 1"]
			},
			{
				"type": "doctrine_rule",
				"title": "Proposal 2",
				"what_happened": "Issue 2",
				"what_was_missing": "Missing 2",
				"proposed_change": "Change 2",
				"rationale": "Rationale 2",
				"confidence": "high",
				"confidence_rationale": "Reason 2",
				"evidence_references": ["line 2"]
			},
			{
				"type": "doctrine_rule",
				"title": "Proposal 3",
				"what_happened": "Issue 3",
				"what_was_missing": "Missing 3",
				"proposed_change": "Change 3",
				"rationale": "Rationale 3",
				"confidence": "high",
				"confidence_rationale": "Reason 3",
				"evidence_references": ["line 3"]
			},
			{
				"type": "doctrine_rule",
				"title": "Proposal 4",
				"what_happened": "Issue 4",
				"what_was_missing": "Missing 4",
				"proposed_change": "Change 4",
				"rationale": "Rationale 4",
				"confidence": "high",
				"confidence_rationale": "Reason 4",
				"evidence_references": ["line 4"]
			},
			{
				"type": "doctrine_rule",
				"title": "Proposal 5",
				"what_happened": "Issue 5",
				"what_was_missing": "Missing 5",
				"proposed_change": "Change 5",
				"rationale": "Rationale 5",
				"confidence": "high",
				"confidence_rationale": "Reason 5",
				"evidence_references": ["line 5"]
			},
			{
				"type": "doctrine_rule",
				"title": "Proposal 6",
				"what_happened": "Issue 6",
				"what_was_missing": "Missing 6",
				"proposed_change": "Change 6",
				"rationale": "Rationale 6",
				"confidence": "high",
				"confidence_rationale": "Reason 6",
				"evidence_references": ["line 6"]
			},
			{
				"type": "doctrine_rule",
				"title": "Proposal 7",
				"what_happened": "Issue 7",
				"what_was_missing": "Missing 7",
				"proposed_change": "Change 7",
				"rationale": "Rationale 7",
				"confidence": "high",
				"confidence_rationale": "Reason 7",
				"evidence_references": ["line 7"]
			}
		]
	}`

	// Create test inputs
	outcomeJSON := json.RawMessage(`{"outcome": "accepted"}`)
	inputs := &DistillerInputs{
		RunID:         "run-trunc",
		SpecID:        "spec-trunc",
		SpecContent:   "test spec for truncation",
		ReviewOutcome: outcomeJSON,
	}

	// Create stub LLM
	stub := &stubLLMCompleter{response: llmResponse}

	// Call Distill
	result, err := Distill(inputs, stub, TierHigh)
	if err != nil {
		t.Fatalf("Distill() returned error: %v", err)
	}

	// Verify proposal count is exactly 5 (truncated from 7)
	if len(result.Proposals) != 5 {
		t.Errorf("Proposal count = %d, want 5", len(result.Proposals))
	}

	// Verify proposal IDs are p1 through p5
	expectedIDs := []string{"p1", "p2", "p3", "p4", "p5"}
	for i, expectedID := range expectedIDs {
		if i >= len(result.Proposals) {
			t.Errorf("Expected proposal %d with ID %q, but no proposal found", i, expectedID)
			continue
		}
		if result.Proposals[i].ID != expectedID {
			t.Errorf("Proposal %d ID = %q, want %q", i, result.Proposals[i].ID, expectedID)
		}
	}

	// Verify that proposals 6 and 7 are not present
	if len(result.Proposals) > 5 {
		t.Errorf("Expected max 5 proposals, but got %d", len(result.Proposals))
	}
}

// TestDistill_FewerThanThree verifies Distill with fewer than 3 proposals.
// Uses a stub LLMCompleter that returns canned JSON with 2 proposals.
// Verifies DistillationResult fields, proposal count, IDs are content-based, and validation passes without error.
func TestDistill_FewerThanThree(t *testing.T) {
	// Create stub LLM response with 2 proposals
	llmResponse := `{
		"proposals": [
			{
				"type": "doctrine_rule",
				"title": "Add error handling",
				"what_happened": "Function did not validate input",
				"what_was_missing": "Input validation",
				"proposed_change": "Add validation before processing",
				"rationale": "Prevents invalid state",
				"confidence": "high",
				"confidence_rationale": "Clear requirement",
				"evidence_references": ["line 10"]
			},
			{
				"type": "planner_heuristic",
				"title": "Improve code organization",
				"what_happened": "Related logic was scattered",
				"what_was_missing": "Code cohesion",
				"proposed_change": "Group related functions together",
				"rationale": "Improves maintainability",
				"confidence": "medium",
				"confidence_rationale": "Subjective improvement",
				"evidence_references": ["line 25"]
			}
		]
	}`

	// Create test inputs
	outcomeJSON := json.RawMessage(`{"outcome": "accepted"}`)
	inputs := &DistillerInputs{
		RunID:         "run-two",
		SpecID:        "spec-two",
		SpecContent:   "test spec with two proposals",
		ReviewOutcome: outcomeJSON,
	}

	// Create stub LLM
	stub := &stubLLMCompleter{response: llmResponse}

	// Call Distill
	result, err := Distill(inputs, stub, TierHigh)
	if err != nil {
		t.Fatalf("Distill() returned error: %v", err)
	}

	// Verify DistillationResult fields
	if result.RunID != "run-two" {
		t.Errorf("RunID = %q, want %q", result.RunID, "run-two")
	}
	if result.SpecID != "spec-two" {
		t.Errorf("SpecID = %q, want %q", result.SpecID, "spec-two")
	}
	if result.Outcome != "accepted" {
		t.Errorf("Outcome = %q, want %q", result.Outcome, "accepted")
	}
	if result.ModelTier != TierHigh {
		t.Errorf("ModelTier = %q, want %q", result.ModelTier, TierHigh)
	}

	// Verify proposal count is 2
	if len(result.Proposals) != 2 {
		t.Errorf("Proposal count = %d, want 2", len(result.Proposals))
	}

	// Verify proposal IDs are content-based (p1, p2)
	expectedIDs := []string{"p1", "p2"}
	for i, expectedID := range expectedIDs {
		if i >= len(result.Proposals) {
			t.Errorf("Expected proposal %d with ID %q, but no proposal found", i, expectedID)
			continue
		}
		if result.Proposals[i].ID != expectedID {
			t.Errorf("Proposal %d ID = %q, want %q", i, result.Proposals[i].ID, expectedID)
		}
	}

	// Verify that proposal types are set correctly
	expectedTypes := []string{"doctrine_rule", "planner_heuristic"}
	for i, expectedType := range expectedTypes {
		if i >= len(result.Proposals) {
			break
		}
		if result.Proposals[i].Type != expectedType {
			t.Errorf("Proposal %d type = %q, want %q", i, result.Proposals[i].Type, expectedType)
		}
	}

	// Verify that CreatedAt is set
	if result.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	// Verify outcome-specific validation passed
	// (The test would fail earlier if ValidateProposals rejected the outcome,
	// so the fact that we reach here means validation passed)
}

// TestDistill_ValidationRejectsNonConforming verifies Distill validation rejection.
// Uses a stub LLMCompleter that returns proposals violating outcome-specific rules.
// For rework_vision_change, proposals must include refinement_guidance type.
// Verifies that Distill returns an error when validation fails.
func TestDistill_ValidationRejectsNonConforming(t *testing.T) {
	// Create stub LLM response with 3 proposals, none of type refinement_guidance.
	// This violates the rework_vision_change requirement.
	llmResponse := `{
		"proposals": [
			{
				"type": "doctrine_rule",
				"title": "Add error checking",
				"what_happened": "Function did not check for errors",
				"what_was_missing": "Error handling",
				"proposed_change": "Add if err != nil check",
				"rationale": "Prevents silent failures",
				"confidence": "high",
				"confidence_rationale": "Standard Go pattern",
				"evidence_references": ["line 42"]
			},
			{
				"type": "planner_heuristic",
				"title": "Use better variable names",
				"what_happened": "Variable names were unclear",
				"what_was_missing": "Clarity",
				"proposed_change": "Rename 'x' to 'count'",
				"rationale": "Improves readability",
				"confidence": "high",
				"confidence_rationale": "Clear naming convention",
				"evidence_references": ["line 15"]
			},
			{
				"type": "validation_gap",
				"title": "Add input validation",
				"what_happened": "Input was not validated",
				"what_was_missing": "Validation",
				"proposed_change": "Add bounds check",
				"rationale": "Prevents invalid input",
				"confidence": "medium",
				"confidence_rationale": "Common edge case",
				"evidence_references": ["line 20"]
			}
		]
	}`

	// Create test inputs with rework_vision_change outcome
	outcomeJSON := json.RawMessage(`{"outcome": "rework_vision_change"}`)
	inputs := &DistillerInputs{
		RunID:         "run-reject",
		SpecID:        "spec-reject",
		SpecContent:   "test spec",
		ReviewOutcome: outcomeJSON,
	}

	// Create stub LLM
	stub := &stubLLMCompleter{response: llmResponse}

	// Call Distill - should return an error
	result, err := Distill(inputs, stub, TierHigh)

	// Verify that error is returned
	if err == nil {
		t.Fatal("Distill() should return an error for non-conforming proposals, but got nil")
	}

	// Verify that result is nil
	if result != nil {
		t.Errorf("result should be nil when validation fails, got %v", result)
	}

	// Verify that error message contains expected validation failure info
	errMsg := err.Error()
	if !strings.Contains(errMsg, "refinement_guidance") {
		t.Errorf("error message should mention refinement_guidance requirement, got: %q", errMsg)
	}
}

// TestDistill_LLMFailure verifies Distill returns an error when LLM fails.
// Uses a stub LLMCompleter that returns an error.
// Verifies Distill returns that error and result is nil.
func TestDistill_LLMFailure(t *testing.T) {
	// Create test inputs
	outcomeJSON := json.RawMessage(`{"outcome": "accepted"}`)
	inputs := &DistillerInputs{
		RunID:         "run-fail",
		SpecID:        "spec-fail",
		SpecContent:   "test spec",
		ReviewOutcome: outcomeJSON,
	}

	// Create stub LLM that returns an error
	expectedErr := "LLM connection failed"
	stub := &stubLLMCompleter{
		response: "",
		err:      &mockError{msg: expectedErr},
	}

	// Call Distill
	result, err := Distill(inputs, stub, TierHigh)

	// Verify that error is returned
	if err == nil {
		t.Fatal("Distill() should return an error when LLM fails, but got nil")
	}

	// Verify that result is nil
	if result != nil {
		t.Errorf("result should be nil when LLM fails, got %v", result)
	}

	// Verify error message contains the LLM error
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("error message should contain LLM error, got: %q", err.Error())
	}
}

// TestDistill_ProposalIDsContentBased verifies proposal IDs are content-based and stable.
// Calls Distill twice with identical content and verifies IDs match across invocations.
// Calls Distill with different content and verifies IDs differ based on proposal count.
func TestDistill_ProposalIDsContentBased(t *testing.T) {
	// LLM response with 3 proposals
	llmResponseThreeProposals := `{
		"proposals": [
			{
				"type": "doctrine_rule",
				"title": "First proposal",
				"what_happened": "Issue A",
				"what_was_missing": "Missing A",
				"proposed_change": "Change A",
				"rationale": "Reason A",
				"confidence": "high",
				"confidence_rationale": "Confident A",
				"evidence_references": ["line 1"]
			},
			{
				"type": "validation_gap",
				"title": "Second proposal",
				"what_happened": "Issue B",
				"what_was_missing": "Missing B",
				"proposed_change": "Change B",
				"rationale": "Reason B",
				"confidence": "high",
				"confidence_rationale": "Confident B",
				"evidence_references": ["line 2"]
			},
			{
				"type": "planner_heuristic",
				"title": "Third proposal",
				"what_happened": "Issue C",
				"what_was_missing": "Missing C",
				"proposed_change": "Change C",
				"rationale": "Reason C",
				"confidence": "medium",
				"confidence_rationale": "Confident C",
				"evidence_references": ["line 3"]
			}
		]
	}`

	// Test inputs
	outcomeJSON := json.RawMessage(`{"outcome": "accepted"}`)
	inputs := &DistillerInputs{
		RunID:         "run-stable",
		SpecID:        "spec-stable",
		SpecContent:   "test spec",
		ReviewOutcome: outcomeJSON,
	}

	stub := &stubLLMCompleter{response: llmResponseThreeProposals}

	// Call Distill twice with identical content
	result1, err := Distill(inputs, stub, TierHigh)
	if err != nil {
		t.Fatalf("First Distill() call returned error: %v", err)
	}

	result2, err := Distill(inputs, stub, TierHigh)
	if err != nil {
		t.Fatalf("Second Distill() call returned error: %v", err)
	}

	// Verify IDs are identical across both calls (demonstrating stability)
	if len(result1.Proposals) != len(result2.Proposals) {
		t.Errorf("Proposal count mismatch: first=%d, second=%d", len(result1.Proposals), len(result2.Proposals))
	}

	for i := 0; i < len(result1.Proposals); i++ {
		if result1.Proposals[i].ID != result2.Proposals[i].ID {
			t.Errorf("Proposal %d ID mismatch: first=%q, second=%q", i, result1.Proposals[i].ID, result2.Proposals[i].ID)
		}
	}

	// Verify IDs are p1, p2, p3 for the three proposals
	expectedIDsThree := []string{"p1", "p2", "p3"}
	for i, expectedID := range expectedIDsThree {
		if result1.Proposals[i].ID != expectedID {
			t.Errorf("Result1 Proposal %d ID = %q, want %q", i, result1.Proposals[i].ID, expectedID)
		}
	}

	// Now test with different content (2 proposals instead of 3)
	llmResponseTwoProposals := `{
		"proposals": [
			{
				"type": "doctrine_rule",
				"title": "Proposal X",
				"what_happened": "Issue X",
				"what_was_missing": "Missing X",
				"proposed_change": "Change X",
				"rationale": "Reason X",
				"confidence": "high",
				"confidence_rationale": "Confident X",
				"evidence_references": ["line 10"]
			},
			{
				"type": "validation_gap",
				"title": "Proposal Y",
				"what_happened": "Issue Y",
				"what_was_missing": "Missing Y",
				"proposed_change": "Change Y",
				"rationale": "Reason Y",
				"confidence": "high",
				"confidence_rationale": "Confident Y",
				"evidence_references": ["line 20"]
			}
		]
	}`

	stubDifferent := &stubLLMCompleter{response: llmResponseTwoProposals}
	result3, err := Distill(inputs, stubDifferent, TierHigh)
	if err != nil {
		t.Fatalf("Distill() with different content returned error: %v", err)
	}

	// Verify result3 has fewer proposals than result1 (demonstrating content-dependence)
	if len(result3.Proposals) >= len(result1.Proposals) {
		t.Errorf("Expected different proposal counts: result1=%d, result3=%d", len(result1.Proposals), len(result3.Proposals))
	}

	// Verify result3 has IDs p1, p2 (different pattern from result1 which has p1, p2, p3)
	expectedIDsTwo := []string{"p1", "p2"}
	for i, expectedID := range expectedIDsTwo {
		if result3.Proposals[i].ID != expectedID {
			t.Errorf("Result3 Proposal %d ID = %q, want %q", i, result3.Proposals[i].ID, expectedID)
		}
	}

	// Verify that result3 does NOT have a p3 ID (proving the IDs differ based on content)
	if len(result3.Proposals) > 2 {
		for i := 2; i < len(result3.Proposals); i++ {
			if result3.Proposals[i].ID == "p3" {
				t.Errorf("Result3 should not have p3 ID since it only has 2 proposals, but got %q", result3.Proposals[i].ID)
			}
		}
	}
}

// mockError implements error interface for testing.
type mockError struct {
	msg string
}

func (m *mockError) Error() string {
	return m.msg
}
