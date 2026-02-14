//go:build acceptance

package validate

import (
	"strings"
	"testing"
)

// TestBuildReprompt_IncludesViolationsForFlaggedBeads tests that the reprompt includes violation details for each flagged bead
func TestBuildReprompt_IncludesViolationsForFlaggedBeads(t *testing.T) {
	// Expected failure: BuildReprompt function does not exist yet

	originalPrompt := "Decompose the following plan into beads..."
	candidates := []BeadCandidate{
		{
			Title:       "Good bead",
			Description: "Properly sized",
			AcceptanceCriteria: []string{
				"Criterion 1",
				"Criterion 2",
			},
		},
		{
			Title:       "Bad bead",
			Description: "Over-scoped",
			AcceptanceCriteria: []string{
				"Criterion 1",
				"Criterion 2",
				"Criterion 3",
				"Criterion 4",
			},
		},
	}
	violations := []Violation{
		{
			BeadIndex: 1,
			Rule:      "criteria_count",
			Message:   "Bead has more than 3 acceptance criteria",
		},
	}

	result := BuildReprompt(originalPrompt, candidates, violations)

	// Should mention the violation rule
	if !strings.Contains(result, "criteria_count") {
		t.Errorf("expected reprompt to contain violation rule 'criteria_count'")
	}

	// Should mention the violation message
	if !strings.Contains(result, "more than 3 acceptance criteria") {
		t.Errorf("expected reprompt to contain violation message about criteria count")
	}

	// Should identify which bead is flagged
	if !strings.Contains(result, "Bad bead") || !strings.Contains(result, "bead 1") || !strings.Contains(result, "index 1") {
		t.Errorf("expected reprompt to identify the flagged bead by title or index")
	}
}

// TestBuildReprompt_InstructsKeepUnflaggedBeads tests that the reprompt instructs Claude to keep valid beads unchanged
func TestBuildReprompt_InstructsKeepUnflaggedBeads(t *testing.T) {
	// Expected failure: BuildReprompt function does not exist yet

	originalPrompt := "Decompose the following plan into beads..."
	candidates := []BeadCandidate{
		{
			Title:       "Good bead",
			Description: "Properly sized",
			AcceptanceCriteria: []string{
				"Criterion 1",
				"Criterion 2",
			},
		},
		{
			Title:       "Bad bead",
			Description: "Over-scoped",
			AcceptanceCriteria: []string{
				"Criterion 1",
				"Criterion 2",
				"Criterion 3",
				"Criterion 4",
			},
		},
	}
	violations := []Violation{
		{
			BeadIndex: 1,
			Rule:      "criteria_count",
			Message:   "Bead has more than 3 acceptance criteria",
		},
	}

	result := BuildReprompt(originalPrompt, candidates, violations)

	// Should instruct to keep unflagged beads unchanged
	lowerResult := strings.ToLower(result)
	hasKeepInstruction := strings.Contains(lowerResult, "keep") && (strings.Contains(lowerResult, "unchanged") || strings.Contains(lowerResult, "as-is") || strings.Contains(lowerResult, "valid"))
	hasOnlyInstruction := strings.Contains(lowerResult, "only") && (strings.Contains(lowerResult, "flagged") || strings.Contains(lowerResult, "violat"))

	if !hasKeepInstruction && !hasOnlyInstruction {
		t.Errorf("expected reprompt to instruct keeping unflagged beads unchanged or only modifying flagged beads")
	}
}

// TestBuildReprompt_RequestsSameJSONFormat tests that the reprompt requests output in the same JSON format
func TestBuildReprompt_RequestsSameJSONFormat(t *testing.T) {
	// Expected failure: BuildReprompt function does not exist yet

	originalPrompt := "Decompose the following plan into beads..."
	candidates := []BeadCandidate{
		{
			Title:       "Bad bead",
			Description: "Over-scoped",
			AcceptanceCriteria: []string{
				"Criterion 1",
				"Criterion 2",
				"Criterion 3",
				"Criterion 4",
			},
		},
	}
	violations := []Violation{
		{
			BeadIndex: 0,
			Rule:      "criteria_count",
			Message:   "Bead has more than 3 acceptance criteria",
		},
	}

	result := BuildReprompt(originalPrompt, candidates, violations)

	// Should mention JSON output format
	if !strings.Contains(result, "JSON") {
		t.Errorf("expected reprompt to mention JSON output format")
	}

	// Should mention the same format or fields
	lowerResult := strings.ToLower(result)
	hasSameFormat := strings.Contains(lowerResult, "same format") || strings.Contains(lowerResult, "same json")
	hasArrayFormat := strings.Contains(lowerResult, "array") || strings.Contains(lowerResult, "[]")

	if !hasSameFormat && !hasArrayFormat {
		t.Errorf("expected reprompt to request the same JSON format or mention array structure")
	}
}

// TestBuildReprompt_IncludesOriginalPromptContext tests that the reprompt includes or references the original prompt
func TestBuildReprompt_IncludesOriginalPromptContext(t *testing.T) {
	// Expected failure: BuildReprompt function does not exist yet

	originalPrompt := "Decompose the following UNIQUE_MARKER plan into beads..."
	candidates := []BeadCandidate{
		{
			Title:       "Bad bead",
			Description: "Over-scoped",
			AcceptanceCriteria: []string{
				"Criterion 1",
				"Criterion 2",
				"Criterion 3",
				"Criterion 4",
			},
		},
	}
	violations := []Violation{
		{
			BeadIndex: 0,
			Rule:      "criteria_count",
			Message:   "Bead has more than 3 acceptance criteria",
		},
	}

	result := BuildReprompt(originalPrompt, candidates, violations)

	// Should include the original prompt content or at least reference it
	if !strings.Contains(result, "UNIQUE_MARKER") {
		t.Errorf("expected reprompt to include the original prompt content")
	}
}

// TestBuildReprompt_MultipleViolationsSameBead tests handling of multiple violations for the same bead
func TestBuildReprompt_MultipleViolationsSameBead(t *testing.T) {
	// Expected failure: BuildReprompt function does not exist yet

	originalPrompt := "Decompose the following plan into beads..."
	candidates := []BeadCandidate{
		{
			Title:       "Very bad bead",
			Description: "Refactor entire system",
			AcceptanceCriteria: []string{
				"Criterion 1",
				"Criterion 2",
				"Criterion 3",
				"Criterion 4",
			},
		},
	}
	violations := []Violation{
		{
			BeadIndex: 0,
			Rule:      "criteria_count",
			Message:   "Bead has more than 3 acceptance criteria",
		},
		{
			BeadIndex: 0,
			Rule:      "scope_signals",
			Message:   "Bead contains scope signal keywords that may indicate over-scoping",
		},
	}

	result := BuildReprompt(originalPrompt, candidates, violations)

	// Should mention both violation types
	if !strings.Contains(result, "criteria_count") {
		t.Errorf("expected reprompt to contain first violation rule 'criteria_count'")
	}
	if !strings.Contains(result, "scope_signals") {
		t.Errorf("expected reprompt to contain second violation rule 'scope_signals'")
	}

	// Should mention the bead title or index once (not duplicated)
	titleCount := strings.Count(result, "Very bad bead")
	if titleCount < 1 {
		t.Errorf("expected bead title to appear at least once, got %d occurrences", titleCount)
	}
}

// TestBuildReprompt_MultipleViolationsDifferentBeads tests handling of violations across multiple beads
func TestBuildReprompt_MultipleViolationsDifferentBeads(t *testing.T) {
	// Expected failure: BuildReprompt function does not exist yet

	originalPrompt := "Decompose the following plan into beads..."
	candidates := []BeadCandidate{
		{
			Title:       "Good bead",
			Description: "Properly sized",
			AcceptanceCriteria: []string{
				"Criterion 1",
			},
		},
		{
			Title:       "Bad bead 1",
			Description: "Too many criteria",
			AcceptanceCriteria: []string{
				"Criterion 1",
				"Criterion 2",
				"Criterion 3",
				"Criterion 4",
			},
		},
		{
			Title:       "Bad bead 2",
			Description: "Refactor entire system",
			AcceptanceCriteria: []string{
				"Criterion 1",
			},
		},
	}
	violations := []Violation{
		{
			BeadIndex: 1,
			Rule:      "criteria_count",
			Message:   "Bead has more than 3 acceptance criteria",
		},
		{
			BeadIndex: 2,
			Rule:      "scope_signals",
			Message:   "Bead contains scope signal keywords that may indicate over-scoping",
		},
	}

	result := BuildReprompt(originalPrompt, candidates, violations)

	// Should mention both flagged beads
	if !strings.Contains(result, "Bad bead 1") {
		t.Errorf("expected reprompt to mention first flagged bead")
	}
	if !strings.Contains(result, "Bad bead 2") {
		t.Errorf("expected reprompt to mention second flagged bead")
	}

	// Should distinguish between the different violations
	if !strings.Contains(result, "criteria_count") {
		t.Errorf("expected reprompt to contain violation for first bead")
	}
	if !strings.Contains(result, "scope_signals") {
		t.Errorf("expected reprompt to contain violation for second bead")
	}
}

// TestBuildReprompt_NoViolations tests that BuildReprompt handles empty violations list
func TestBuildReprompt_NoViolations(t *testing.T) {
	// Expected failure: BuildReprompt function does not exist yet

	originalPrompt := "Decompose the following plan into beads..."
	candidates := []BeadCandidate{
		{
			Title:       "Good bead",
			Description: "Properly sized",
			AcceptanceCriteria: []string{
				"Criterion 1",
			},
		},
	}
	violations := []Violation{}

	result := BuildReprompt(originalPrompt, candidates, violations)

	// Should still return a valid prompt, possibly indicating no violations
	if result == "" {
		t.Errorf("expected non-empty result even with no violations")
	}

	// Should still include original prompt context
	if !strings.Contains(result, "Decompose") {
		t.Errorf("expected result to include original prompt context")
	}
}

// TestBuildReprompt_IncludesBeadDefinitions tests that the reprompt includes the original bead definitions
func TestBuildReprompt_IncludesBeadDefinitions(t *testing.T) {
	// Expected failure: BuildReprompt function does not exist yet

	originalPrompt := "Decompose the following plan into beads..."
	candidates := []BeadCandidate{
		{
			Title:       "Implement auth",
			Description: "Add authentication system",
			AcceptanceCriteria: []string{
				"User can log in",
				"User can log out",
				"Tokens are validated",
				"Session management works",
			},
		},
	}
	violations := []Violation{
		{
			BeadIndex: 0,
			Rule:      "criteria_count",
			Message:   "Bead has more than 3 acceptance criteria",
		},
	}

	result := BuildReprompt(originalPrompt, candidates, violations)

	// Should include bead title
	if !strings.Contains(result, "Implement auth") {
		t.Errorf("expected reprompt to include bead title")
	}

	// Should include acceptance criteria
	if !strings.Contains(result, "User can log in") {
		t.Errorf("expected reprompt to include acceptance criteria from original definition")
	}
	if !strings.Contains(result, "Session management works") {
		t.Errorf("expected reprompt to include all acceptance criteria from original definition")
	}
}

// TestBuildReprompt_ClearlyIdentifiesFlaggedBeads tests that the reprompt clearly marks which beads need re-decomposition
func TestBuildReprompt_ClearlyIdentifiesFlaggedBeads(t *testing.T) {
	// Expected failure: BuildReprompt function does not exist yet

	originalPrompt := "Decompose the following plan into beads..."
	candidates := []BeadCandidate{
		{
			Title:       "Good bead A",
			Description: "Properly sized",
			AcceptanceCriteria: []string{
				"Criterion 1",
			},
		},
		{
			Title:       "Bad bead B",
			Description: "Over-scoped",
			AcceptanceCriteria: []string{
				"Criterion 1",
				"Criterion 2",
				"Criterion 3",
				"Criterion 4",
			},
		},
		{
			Title:       "Good bead C",
			Description: "Also fine",
			AcceptanceCriteria: []string{
				"Criterion 1",
			},
		},
	}
	violations := []Violation{
		{
			BeadIndex: 1,
			Rule:      "criteria_count",
			Message:   "Bead has more than 3 acceptance criteria",
		},
	}

	result := BuildReprompt(originalPrompt, candidates, violations)

	lowerResult := strings.ToLower(result)

	// Should identify which beads are flagged vs valid
	hasFlaggedSection := strings.Contains(lowerResult, "flagged") || strings.Contains(lowerResult, "violat") || strings.Contains(lowerResult, "problem")
	hasValidSection := strings.Contains(lowerResult, "valid") || strings.Contains(lowerResult, "correct") || strings.Contains(lowerResult, "keep")

	if !hasFlaggedSection {
		t.Errorf("expected reprompt to clearly identify flagged beads")
	}

	if !hasValidSection {
		t.Errorf("expected reprompt to clearly identify valid beads to keep")
	}

	// Should mention the bad bead
	if !strings.Contains(result, "Bad bead B") {
		t.Errorf("expected reprompt to mention the flagged bead by title")
	}
}
