package reviewpacket

import (
	"testing"
)

func TestGenerator_ReadyForReview(t *testing.T) {
	// Scenario: clean run produces full review packet
	// Given: a run reached ready_for_review with all validation passed, all acceptance criteria passed,
	// no blocking review findings, and no degraded flags
	// When: the finalize stage completes
	// Then: the evidence directory contains all five new artifacts; product-review.json has is_diagnostic: false,
	// behavior cards with automatic_status: "proven", and no surprises; process-review.json has trust_level: "high"
	// and recommended_posture: "quick_accept_path"; manual-checklist.json has one item per scenario

	g := &Generator{}

	inputs := Inputs{
		RunID:         "test-run-1",
		SpecTitle:     "Test Spec",
		SpecContent:   readyForReviewSpec(),
		TerminalState: "ready_for_review",
		ValidationResult: ValidationData{
			Passed: true,
			Checks: 12,
		},
		ReviewFindings: map[string][]ReviewFinding{
			"info": {},
		},
		AcceptanceResult: AcceptanceData{
			Passed: 5,
			Failed: 0,
			Unclear: 0,
		},
		DegradedFlags:   []string{},
		RepairCycles:    0,
		RepeatedFailure: false,
	}

	outputs, err := g.Generate(inputs)
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	// Check ProductReview
	if outputs.ProductReview.RunID != "test-run-1" {
		t.Errorf("ProductReview.RunID = %q, want %q", outputs.ProductReview.RunID, "test-run-1")
	}
	if outputs.ProductReview.TerminalState != "ready_for_review" {
		t.Errorf("ProductReview.TerminalState = %q, want %q", outputs.ProductReview.TerminalState, "ready_for_review")
	}
	if outputs.ProductReview.IsDiagnostic {
		t.Errorf("ProductReview.IsDiagnostic = true, want false")
	}
	if outputs.ProductReview.Summary == "" {
		t.Errorf("ProductReview.Summary is empty, want non-empty")
	}

	// Should have 2 behavior cards (from 2 scenarios)
	if len(outputs.ProductReview.BehaviorCards) != 2 {
		t.Errorf("len(BehaviorCards) = %d, want 2", len(outputs.ProductReview.BehaviorCards))
	}

	// All cards should be "proven" status
	for i, card := range outputs.ProductReview.BehaviorCards {
		if card.AutomaticStatus != "proven" {
			t.Errorf("BehaviorCard[%d].AutomaticStatus = %q, want %q", i, card.AutomaticStatus, "proven")
		}
	}

	// Should have no surprises in a clean run
	if len(outputs.ProductReview.Surprises) > 0 {
		t.Errorf("ProductReview.Surprises = %v, want empty", outputs.ProductReview.Surprises)
	}

	// Check ProcessReview
	if outputs.ProcessReview.TrustLevel != "high" {
		t.Errorf("ProcessReview.TrustLevel = %q, want %q", outputs.ProcessReview.TrustLevel, "high")
	}
	if outputs.ProcessReview.RecommendedPosture != "quick_accept_path" {
		t.Errorf("ProcessReview.RecommendedPosture = %q, want %q", outputs.ProcessReview.RecommendedPosture, "quick_accept_path")
	}
	if outputs.ProcessReview.AutomaticProof == "" {
		t.Errorf("ProcessReview.AutomaticProof is empty, want non-empty")
	}
	if outputs.ProcessReview.Acceptance == "" {
		t.Errorf("ProcessReview.Acceptance is empty, want non-empty")
	}

	// Check ManualChecklist
	if len(outputs.ManualChecklist.Items) != 2 {
		t.Errorf("len(ManualChecklist.Items) = %d, want 2", len(outputs.ManualChecklist.Items))
	}

	for i, item := range outputs.ManualChecklist.Items {
		if item.ID == "" {
			t.Errorf("ManualCheckItem[%d].ID is empty", i)
		}
		if item.Title == "" {
			t.Errorf("ManualCheckItem[%d].Title is empty", i)
		}
	}
}

func TestGenerator_Blocked(t *testing.T) {
	// Scenario: blocked run produces diagnostic packet
	// Given: a run ended in blocked
	// When: the finalize stage completes
	// Then: product-review.json has is_diagnostic: true, a populated blocker_summary,
	// a populated recommended_next_action, and behavior cards with automatic_status: "unclear";
	// process-review.json has trust_level: "low"; manual-checklist.json has an empty items array

	g := &Generator{}

	inputs := Inputs{
		RunID:         "test-blocked",
		SpecTitle:     "Test Spec",
		SpecContent:   readyForReviewSpec(),
		TerminalState: "blocked",
		ValidationResult: ValidationData{
			Passed: false,
			Checks: 5,
		},
		ReviewFindings: map[string][]ReviewFinding{
			"blocking": {{Message: "build failed"}},
		},
		AcceptanceResult: AcceptanceData{
			Passed:  0,
			Failed:  0,
			Unclear: 0,
		},
		DegradedFlags:   []string{},
		RepairCycles:    3,
		RepeatedFailure: false,
	}

	outputs, err := g.Generate(inputs)
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	// Check ProductReview diagnostic variant
	if !outputs.ProductReview.IsDiagnostic {
		t.Errorf("ProductReview.IsDiagnostic = false, want true")
	}
	if outputs.ProductReview.BlockerSummary == "" {
		t.Errorf("ProductReview.BlockerSummary is empty, want non-empty")
	}
	if outputs.ProductReview.RecommendedNextAction == "" {
		t.Errorf("ProductReview.RecommendedNextAction is empty, want non-empty")
	}

	// All cards should be "unclear" for blocked runs
	for i, card := range outputs.ProductReview.BehaviorCards {
		if card.AutomaticStatus != "unclear" {
			t.Errorf("BehaviorCard[%d].AutomaticStatus = %q, want %q", i, card.AutomaticStatus, "unclear")
		}
	}

	// Check ProcessReview for blocked
	if outputs.ProcessReview.TrustLevel != "low" {
		t.Errorf("ProcessReview.TrustLevel = %q, want %q", outputs.ProcessReview.TrustLevel, "low")
	}

	// Manual checklist should be empty for diagnostic runs
	if len(outputs.ManualChecklist.Items) != 0 {
		t.Errorf("len(ManualChecklist.Items) = %d, want 0", len(outputs.ManualChecklist.Items))
	}
}

func TestGenerator_NoScenarios_FallsBackToCriteria(t *testing.T) {
	// Scenario: spec with no scenarios falls back to acceptance criteria
	// Given: a spec has 3 acceptance criteria but no ### Scenario: blocks, and the run reached ready_for_review
	// When: the finalize stage completes
	// Then: product-review.json contains 3 behavior cards, one per acceptance criterion,
	// with titles derived from criterion text; manual-checklist.json has 3 items derived from the same criteria

	g := &Generator{}

	inputs := Inputs{
		RunID:         "test-no-scenarios",
		SpecTitle:     "Test Spec No Scenarios",
		SpecContent:   noScenarioSpec(),
		TerminalState: "ready_for_review",
		ValidationResult: ValidationData{
			Passed: true,
			Checks: 10,
		},
		ReviewFindings: map[string][]ReviewFinding{
			"info": {},
		},
		AcceptanceResult: AcceptanceData{
			Passed:  3,
			Failed:  0,
			Unclear: 0,
		},
		DegradedFlags:   []string{},
		RepairCycles:    0,
		RepeatedFailure: false,
	}

	outputs, err := g.Generate(inputs)
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	// Should have 3 behavior cards (from 3 acceptance criteria)
	if len(outputs.ProductReview.BehaviorCards) != 3 {
		t.Errorf("len(BehaviorCards) = %d, want 3", len(outputs.ProductReview.BehaviorCards))
	}

	// Check manual checklist has 3 items
	if len(outputs.ManualChecklist.Items) != 3 {
		t.Errorf("len(ManualChecklist.Items) = %d, want 3", len(outputs.ManualChecklist.Items))
	}
}

func TestGenerator_Degraded(t *testing.T) {
	// Scenario: degraded run produces medium-trust packet
	// Given: a run reached ready_for_review with validation passed and acceptance passed,
	// but the diff was unavailable during review (degraded flag diff_unavailable)
	// When: the finalize stage completes
	// Then: process-review.json has trust_level: "medium", degraded_flags: ["diff_unavailable"],
	// and recommended_posture: "manual_check_carefully"; behavior cards have automatic_status: "mixed"

	g := &Generator{}

	inputs := Inputs{
		RunID:         "test-degraded",
		SpecTitle:     "Test Spec",
		SpecContent:   readyForReviewSpec(),
		TerminalState: "ready_for_review",
		ValidationResult: ValidationData{
			Passed: true,
			Checks: 12,
		},
		ReviewFindings: map[string][]ReviewFinding{
			"info": {},
		},
		AcceptanceResult: AcceptanceData{
			Passed:  5,
			Failed:  0,
			Unclear: 0,
		},
		DegradedFlags:   []string{"diff_unavailable"},
		RepairCycles:    0,
		RepeatedFailure: false,
	}

	outputs, err := g.Generate(inputs)
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	// Check ProcessReview for degraded
	if outputs.ProcessReview.TrustLevel != "medium" {
		t.Errorf("ProcessReview.TrustLevel = %q, want %q", outputs.ProcessReview.TrustLevel, "medium")
	}
	if outputs.ProcessReview.RecommendedPosture != "manual_check_carefully" {
		t.Errorf("ProcessReview.RecommendedPosture = %q, want %q", outputs.ProcessReview.RecommendedPosture, "manual_check_carefully")
	}
	if len(outputs.ProcessReview.DegradedFlags) != 1 || outputs.ProcessReview.DegradedFlags[0] != "diff_unavailable" {
		t.Errorf("ProcessReview.DegradedFlags = %v, want [diff_unavailable]", outputs.ProcessReview.DegradedFlags)
	}

	// All cards should be "mixed" status for degraded runs
	for i, card := range outputs.ProductReview.BehaviorCards {
		if card.AutomaticStatus != "mixed" {
			t.Errorf("BehaviorCard[%d].AutomaticStatus = %q, want %q", i, card.AutomaticStatus, "mixed")
		}
	}
}

func TestGenerator_NeedsHuman(t *testing.T) {
	// Similar to blocked, but needs_human terminal state
	g := &Generator{}

	inputs := Inputs{
		RunID:         "test-needs-human",
		SpecTitle:     "Test Spec",
		SpecContent:   readyForReviewSpec(),
		TerminalState: "needs_human",
		ValidationResult: ValidationData{
			Passed: true,
			Checks: 12,
		},
		ReviewFindings: map[string][]ReviewFinding{
			"info": {},
		},
		AcceptanceResult: AcceptanceData{
			Passed:  5,
			Failed:  0,
			Unclear: 0,
		},
		DegradedFlags:   []string{},
		RepairCycles:    0,
		RepeatedFailure: false,
	}

	outputs, err := g.Generate(inputs)
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	// Check diagnostic variant
	if !outputs.ProductReview.IsDiagnostic {
		t.Errorf("ProductReview.IsDiagnostic = false, want true")
	}
	if outputs.ProcessReview.TrustLevel != "low" {
		t.Errorf("ProcessReview.TrustLevel = %q, want %q", outputs.ProcessReview.TrustLevel, "low")
	}
	if len(outputs.ManualChecklist.Items) != 0 {
		t.Errorf("len(ManualChecklist.Items) = %d, want 0", len(outputs.ManualChecklist.Items))
	}
}

// Test helper specs
func readyForReviewSpec() string {
	return `# Test Spec

## Scenarios

### Scenario: user logs in successfully
Given a user with valid credentials
When they submit the login form
Then they are redirected to the dashboard

### Scenario: user sees error with invalid credentials
Given a user with invalid credentials
When they submit the login form
Then an error message is displayed

## Acceptance Criteria
1. Login form accepts email and password
2. Form validation works before submission
3. Successful login redirects to dashboard
4. Invalid credentials show error
5. Session token is set on successful login

## Validation
### Automatic
Some validation checks
`
}

func noScenarioSpec() string {
	return `# Test Spec No Scenarios

## Acceptance Criteria
1. Feature A should work correctly
2. Feature B should be performant
3. Feature C should handle errors gracefully

## Validation
### Automatic
Some validation checks
`
}
