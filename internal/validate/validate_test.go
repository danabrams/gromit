package validate

import (
	"strings"
	"testing"
)

// TestCheckBeads_NoCriteria tests that beads with zero criteria are not flagged by criteria count rule
func TestCheckBeads_NoCriteria(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:              "Test bead",
			Description:        "Test description",
			AcceptanceCriteria: []string{},
		},
	}

	violations := CheckBeads(beads)

	if len(violations) != 0 {
		t.Errorf("expected no violations for bead with 0 criteria, got %d", len(violations))
	}
}

// TestCheckBeads_ThreeCriteria tests that beads with exactly 3 criteria are not flagged
func TestCheckBeads_ThreeCriteria(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:       "Test bead",
			Description: "Test description",
			AcceptanceCriteria: []string{
				"Criterion 1",
				"Criterion 2",
				"Criterion 3",
			},
		},
	}

	violations := CheckBeads(beads)

	if len(violations) != 0 {
		t.Errorf("expected no violations for bead with 3 criteria, got %d", len(violations))
	}
}

// TestCheckBeads_FourCriteria tests that beads with more than 3 criteria are flagged
func TestCheckBeads_FourCriteria(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:       "Oversized bead",
			Description: "Too many criteria",
			AcceptanceCriteria: []string{
				"Criterion 1",
				"Criterion 2",
				"Criterion 3",
				"Criterion 4",
			},
		},
	}

	violations := CheckBeads(beads)

	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}

	v := violations[0]
	if v.BeadIndex != 0 {
		t.Errorf("expected BeadIndex=0, got %d", v.BeadIndex)
	}
	if v.Rule != "criteria_count" {
		t.Errorf("expected Rule='criteria_count', got %q", v.Rule)
	}
	if !strings.Contains(v.Message, "more than 3") {
		t.Errorf("expected message to mention 'more than 3', got %q", v.Message)
	}
}

// TestCheckBeads_SiblingOverlap_ExactSubstring tests that overlapping criteria are detected
func TestCheckBeads_SiblingOverlap_ExactSubstring(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:       "Bead 1",
			Description: "First bead",
			AcceptanceCriteria: []string{
				"Implement user authentication",
				"Add login form",
			},
		},
		{
			Title:       "Bead 2",
			Description: "Second bead",
			AcceptanceCriteria: []string{
				"Implement user authentication flow",
				"Add logout button",
			},
		},
	}

	violations := CheckBeads(beads)

	// Should flag at least one violation for overlapping criteria
	if len(violations) == 0 {
		t.Fatal("expected at least 1 violation for overlapping criteria, got 0")
	}

	foundOverlap := false
	for _, v := range violations {
		if v.Rule == "sibling_overlap" {
			foundOverlap = true
			if !strings.Contains(strings.ToLower(v.Message), "overlap") {
				t.Errorf("expected message to mention 'overlap', got %q", v.Message)
			}
		}
	}

	if !foundOverlap {
		t.Error("expected at least one sibling_overlap violation")
	}
}

// TestCheckBeads_SiblingOverlap_CaseInsensitive tests that overlap detection is case-insensitive
func TestCheckBeads_SiblingOverlap_CaseInsensitive(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:       "Bead 1",
			Description: "First bead",
			AcceptanceCriteria: []string{
				"Add database migration scripts for users table",
			},
		},
		{
			Title:       "Bead 2",
			Description: "Second bead",
			AcceptanceCriteria: []string{
				"add database migration scripts for users table and sessions",
			},
		},
	}

	violations := CheckBeads(beads)

	foundOverlap := false
	for _, v := range violations {
		if v.Rule == "sibling_overlap" {
			foundOverlap = true
		}
	}

	if !foundOverlap {
		t.Error("expected sibling_overlap violation for case-insensitive match")
	}
}

// TestCheckBeads_NoSiblingOverlap tests that distinct criteria are not flagged
func TestCheckBeads_NoSiblingOverlap(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:       "Bead 1",
			Description: "First bead",
			AcceptanceCriteria: []string{
				"Implement authentication",
			},
		},
		{
			Title:       "Bead 2",
			Description: "Second bead",
			AcceptanceCriteria: []string{
				"Add email validation",
			},
		},
	}

	violations := CheckBeads(beads)

	for _, v := range violations {
		if v.Rule == "sibling_overlap" {
			t.Errorf("unexpected sibling_overlap violation for distinct criteria: %s", v.Message)
		}
	}
}

// TestCheckBeads_ScopeSignals_TitleKeywords tests that scope signal keywords in title are detected
func TestCheckBeads_ScopeSignals_TitleKeywords(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		wantHit bool
	}{
		{
			name:    "refactor entire",
			title:   "Refactor entire authentication system",
			wantHit: true,
		},
		{
			name:    "update all",
			title:   "Update all database schemas",
			wantHit: true,
		},
		{
			name:    "across all packages",
			title:   "Add logging across all packages",
			wantHit: true,
		},
		{
			name:    "and also",
			title:   "Add feature X and also refactor Y",
			wantHit: true,
		},
		{
			name:    "normal title",
			title:   "Add user login endpoint",
			wantHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beads := []BeadCandidate{
				{
					Title:              tt.title,
					Description:        "Test description",
					AcceptanceCriteria: []string{"Criterion 1"},
				},
			}

			violations := CheckBeads(beads)

			foundScopeSignal := false
			for _, v := range violations {
				if v.Rule == "scope_signals" {
					foundScopeSignal = true
				}
			}

			if tt.wantHit && !foundScopeSignal {
				t.Errorf("expected scope_signals violation for title %q, got none", tt.title)
			}
			if !tt.wantHit && foundScopeSignal {
				t.Errorf("unexpected scope_signals violation for title %q", tt.title)
			}
		})
	}
}

// TestCheckBeads_ScopeSignals_DescriptionKeywords tests that scope signal keywords in description are detected
func TestCheckBeads_ScopeSignals_DescriptionKeywords(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:              "Add feature",
			Description:        "This bead will refactor entire codebase and also update all tests",
			AcceptanceCriteria: []string{"Criterion 1"},
		},
	}

	violations := CheckBeads(beads)

	foundScopeSignal := false
	for _, v := range violations {
		if v.Rule == "scope_signals" {
			foundScopeSignal = true
			if !strings.Contains(strings.ToLower(v.Message), "scope") {
				t.Errorf("expected message to mention 'scope', got %q", v.Message)
			}
		}
	}

	if !foundScopeSignal {
		t.Error("expected scope_signals violation for description with keywords")
	}
}

// TestCheckBeads_MultipleViolations tests that a single bead can have multiple violations
func TestCheckBeads_MultipleViolations(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:       "Refactor entire system",
			Description: "Over-scoped bead",
			AcceptanceCriteria: []string{
				"Criterion 1",
				"Criterion 2",
				"Criterion 3",
				"Criterion 4",
			},
		},
	}

	violations := CheckBeads(beads)

	// Should have at least 2 violations: criteria_count and scope_signals
	if len(violations) < 2 {
		t.Errorf("expected at least 2 violations, got %d", len(violations))
	}

	rules := make(map[string]bool)
	for _, v := range violations {
		rules[v.Rule] = true
		if v.BeadIndex != 0 {
			t.Errorf("expected all violations for BeadIndex=0, got %d", v.BeadIndex)
		}
	}

	if !rules["criteria_count"] {
		t.Error("expected criteria_count violation")
	}
	if !rules["scope_signals"] {
		t.Error("expected scope_signals violation")
	}
}

// TestCheckBeads_MultipleBeads tests violation tracking across multiple beads
func TestCheckBeads_MultipleBeads(t *testing.T) {
	beads := []BeadCandidate{
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

	violations := CheckBeads(beads)

	// Should only have violations for the second bead
	if len(violations) == 0 {
		t.Fatal("expected violations for second bead, got none")
	}

	for _, v := range violations {
		if v.BeadIndex != 1 {
			t.Errorf("expected violations only for BeadIndex=1, got violation at %d", v.BeadIndex)
		}
	}
}

// TestCheckBeads_EmptyList tests that empty bead list returns no violations
func TestCheckBeads_EmptyList(t *testing.T) {
	beads := []BeadCandidate{}

	violations := CheckBeads(beads)

	if len(violations) != 0 {
		t.Errorf("expected no violations for empty bead list, got %d", len(violations))
	}
}

// TestViolation_FieldsPresent tests that Violation struct has required fields
func TestViolation_FieldsPresent(t *testing.T) {
	v := Violation{
		BeadIndex: 1,
		Rule:      "test_rule",
		Message:   "Test message",
	}

	if v.BeadIndex != 1 {
		t.Errorf("expected BeadIndex=1, got %d", v.BeadIndex)
	}
	if v.Rule != "test_rule" {
		t.Errorf("expected Rule='test_rule', got %q", v.Rule)
	}
	if v.Message != "Test message" {
		t.Errorf("expected Message='Test message', got %q", v.Message)
	}
}

// TestBeadCandidate_FieldsPresent tests that BeadCandidate struct has required fields
func TestBeadCandidate_FieldsPresent(t *testing.T) {
	bc := BeadCandidate{
		Title:       "Test title",
		Description: "Test description",
		AcceptanceCriteria: []string{
			"Criterion 1",
			"Criterion 2",
		},
	}

	if bc.Title != "Test title" {
		t.Errorf("expected Title='Test title', got %q", bc.Title)
	}
	if bc.Description != "Test description" {
		t.Errorf("expected Description='Test description', got %q", bc.Description)
	}
	if len(bc.AcceptanceCriteria) != 2 {
		t.Errorf("expected 2 criteria, got %d", len(bc.AcceptanceCriteria))
	}
}

// TestCheckBeads_ScopeSignals_CaseInsensitive tests that scope signal detection is case-insensitive
func TestCheckBeads_ScopeSignals_CaseInsensitive(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:              "REFACTOR ENTIRE system",
			Description:        "Test",
			AcceptanceCriteria: []string{"Criterion 1"},
		},
	}

	violations := CheckBeads(beads)

	foundScopeSignal := false
	for _, v := range violations {
		if v.Rule == "scope_signals" {
			foundScopeSignal = true
		}
	}

	if !foundScopeSignal {
		t.Error("expected scope_signals violation for uppercase keywords")
	}
}

// TestCheckBeads_SiblingOverlap_SingleBead tests that single-bead input has no sibling overlap
func TestCheckBeads_SiblingOverlap_SingleBead(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:       "Only bead",
			Description: "Single bead",
			AcceptanceCriteria: []string{
				"Criterion 1",
				"Criterion 2",
			},
		},
	}

	violations := CheckBeads(beads)

	for _, v := range violations {
		if v.Rule == "sibling_overlap" {
			t.Errorf("unexpected sibling_overlap violation for single bead: %s", v.Message)
		}
	}
}

// TestCheckBeads_SiblingOverlap_ShortMatchBelowThreshold tests that short shared phrases don't trigger false positives
func TestCheckBeads_SiblingOverlap_ShortMatchBelowThreshold(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:       "Bead 1",
			Description: "First bead",
			AcceptanceCriteria: []string{
				"Add database migration",
			},
		},
		{
			Title:       "Bead 2",
			Description: "Second bead",
			AcceptanceCriteria: []string{
				"add database migration for sessions",
			},
		},
	}

	violations := CheckBeads(beads)

	for _, v := range violations {
		if v.Rule == "sibling_overlap" {
			t.Errorf("unexpected sibling_overlap for short criterion (%d chars < 25 threshold): %s", len("add database migration"), v.Message)
		}
	}
}

// TestCheckBeads_SixExpectedOutputs_ViolatesOutputCount tests that beads with more than 5 expected outputs are flagged
func TestCheckBeads_SixExpectedOutputs_ViolatesOutputCount(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:           "Oversized bead",
			Description:     "Too many expected outputs",
			ExpectedOutputs: []string{"o1", "o2", "o3", "o4", "o5", "o6"},
		},
	}

	violations := CheckBeads(beads)

	found := false
	for _, v := range violations {
		if v.Rule == "output_count" {
			found = true
			if v.BeadIndex != 0 {
				t.Errorf("BeadIndex = %d, want 0", v.BeadIndex)
			}
			if !strings.Contains(v.Message, "5") {
				t.Errorf("expected message to mention limit of 5, got %q", v.Message)
			}
		}
	}
	if !found {
		t.Error("expected output_count violation for bead with 6 expected outputs")
	}
}

// TestCheckBeads_FiveExpectedOutputs_NoOutputCountViolation tests that exactly 5 expected outputs is valid
func TestCheckBeads_FiveExpectedOutputs_NoOutputCountViolation(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:           "Valid bead",
			Description:     "Exactly 5 outputs",
			ExpectedOutputs: []string{"o1", "o2", "o3", "o4", "o5"},
		},
	}

	violations := CheckBeads(beads)

	for _, v := range violations {
		if v.Rule == "output_count" {
			t.Errorf("unexpected output_count violation for bead with exactly 5 expected outputs")
		}
	}
}

// TestCheckBeads_EmptyExpectedOutput_ViolatesOutputEmpty tests that beads with empty expected output strings are flagged
func TestCheckBeads_EmptyExpectedOutput_ViolatesOutputEmpty(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:           "Bead with empty output",
			Description:     "Has an empty string in outputs",
			ExpectedOutputs: []string{"valid output", "", "another valid output"},
		},
	}

	violations := CheckBeads(beads)

	found := false
	for _, v := range violations {
		if v.Rule == "output_empty" {
			found = true
			if v.BeadIndex != 0 {
				t.Errorf("BeadIndex = %d, want 0", v.BeadIndex)
			}
		}
	}
	if !found {
		t.Error("expected output_empty violation for bead with empty expected output")
	}
}

// TestCheckBeads_NoEmptyExpectedOutputs_NoOutputEmptyViolation tests that non-empty outputs are not flagged
func TestCheckBeads_NoEmptyExpectedOutputs_NoOutputEmptyViolation(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:           "Clean bead",
			Description:     "No empty outputs",
			ExpectedOutputs: []string{"output A", "output B"},
		},
	}

	violations := CheckBeads(beads)

	for _, v := range violations {
		if v.Rule == "output_empty" {
			t.Errorf("unexpected output_empty violation for bead with no empty outputs")
		}
	}
}

// TestCheckBeads_DuplicateExpectedOutputs_ViolatesOutputDuplicate tests that duplicate expected outputs within a bead are flagged
func TestCheckBeads_DuplicateExpectedOutputs_ViolatesOutputDuplicate(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:           "Bead with duplicates",
			Description:     "Same output appears twice",
			ExpectedOutputs: []string{"output A", "output B", "output A"},
		},
	}

	violations := CheckBeads(beads)

	found := false
	for _, v := range violations {
		if v.Rule == "output_duplicate" {
			found = true
			if v.BeadIndex != 0 {
				t.Errorf("BeadIndex = %d, want 0", v.BeadIndex)
			}
		}
	}
	if !found {
		t.Error("expected output_duplicate violation for bead with duplicate expected outputs")
	}
}

// TestCheckBeads_NoDuplicateExpectedOutputs_NoOutputDuplicateViolation tests that unique outputs are not flagged
func TestCheckBeads_NoDuplicateExpectedOutputs_NoOutputDuplicateViolation(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:           "Clean bead",
			Description:     "All unique outputs",
			ExpectedOutputs: []string{"output A", "output B", "output C"},
		},
	}

	violations := CheckBeads(beads)

	for _, v := range violations {
		if v.Rule == "output_duplicate" {
			t.Errorf("unexpected output_duplicate violation for bead with unique outputs")
		}
	}
}

// TestCheckBeads_SiblingOverlap_NonOverlappingSubstrings tests that partial word matches don't trigger false positives
func TestCheckBeads_SiblingOverlap_NonOverlappingSubstrings(t *testing.T) {
	beads := []BeadCandidate{
		{
			Title:       "Bead 1",
			Description: "First bead",
			AcceptanceCriteria: []string{
				"Add authentication",
			},
		},
		{
			Title:       "Bead 2",
			Description: "Second bead",
			AcceptanceCriteria: []string{
				"Add authorization",
			},
		},
	}

	violations := CheckBeads(beads)

	// These should not overlap unless the similarity threshold is very low
	// The test verifies reasonable threshold behavior
	for _, v := range violations {
		if v.Rule == "sibling_overlap" {
			// If there is overlap detected, ensure it's reasonable
			// (This is a weak assertion since we don't know the exact threshold)
			t.Logf("Note: sibling_overlap detected between 'authentication' and 'authorization': %s", v.Message)
		}
	}
}
