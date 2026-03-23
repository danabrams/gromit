package proposaltriage

import (
	"testing"
	"time"
)

func TestLoadDecisions_MissingFileReturnsEmptySlice(t *testing.T) {
	tmpDir := t.TempDir()

	decisions, err := LoadDecisions(tmpDir)

	if err != nil {
		t.Fatalf("LoadDecisions should not error on missing file, got: %v", err)
	}
	if decisions == nil {
		t.Fatal("LoadDecisions should return empty slice, not nil")
	}
	if len(decisions) != 0 {
		t.Fatalf("LoadDecisions should return empty slice, got %d decisions", len(decisions))
	}
}

func TestSaveDecisions_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test decisions
	original := []Decision{
		{
			ProposalID:        "prop-1",
			Action:            "accepted",
			Reason:            "looks good",
			ApprovedTitle:     "Title 1",
			ApprovedChange:    "Change 1",
			ApprovedRationale: "Rationale 1",
			DecidedAt:         time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
		},
		{
			ProposalID:  "prop-2",
			Action:      "rejected",
			Reason:      "needs work",
			DuplicateOf: "prop-3",
			DecidedAt:   time.Date(2026, 3, 21, 11, 0, 0, 0, time.UTC),
		},
	}

	// Save decisions
	if err := SaveDecisions(tmpDir, original); err != nil {
		t.Fatalf("SaveDecisions failed: %v", err)
	}

	// Load decisions
	loaded, err := LoadDecisions(tmpDir)
	if err != nil {
		t.Fatalf("LoadDecisions failed: %v", err)
	}

	// Verify round-trip
	if len(loaded) != len(original) {
		t.Fatalf("loaded %d decisions, expected %d", len(loaded), len(original))
	}

	for i, d := range loaded {
		if d.ProposalID != original[i].ProposalID {
			t.Errorf("decision %d: ProposalID mismatch, got %q, want %q", i, d.ProposalID, original[i].ProposalID)
		}
		if d.Action != original[i].Action {
			t.Errorf("decision %d: Action mismatch, got %q, want %q", i, d.Action, original[i].Action)
		}
		if d.Reason != original[i].Reason {
			t.Errorf("decision %d: Reason mismatch, got %q, want %q", i, d.Reason, original[i].Reason)
		}
		if d.ApprovedTitle != original[i].ApprovedTitle {
			t.Errorf("decision %d: ApprovedTitle mismatch, got %q, want %q", i, d.ApprovedTitle, original[i].ApprovedTitle)
		}
		if d.ApprovedChange != original[i].ApprovedChange {
			t.Errorf("decision %d: ApprovedChange mismatch, got %q, want %q", i, d.ApprovedChange, original[i].ApprovedChange)
		}
		if d.ApprovedRationale != original[i].ApprovedRationale {
			t.Errorf("decision %d: ApprovedRationale mismatch, got %q, want %q", i, d.ApprovedRationale, original[i].ApprovedRationale)
		}
		if d.MaterializedID != original[i].MaterializedID {
			t.Errorf("decision %d: MaterializedID mismatch, got %q, want %q", i, d.MaterializedID, original[i].MaterializedID)
		}
		if d.DuplicateOf != original[i].DuplicateOf {
			t.Errorf("decision %d: DuplicateOf mismatch, got %q, want %q", i, d.DuplicateOf, original[i].DuplicateOf)
		}
		if !d.DecidedAt.Equal(original[i].DecidedAt) {
			t.Errorf("decision %d: DecidedAt mismatch, got %v, want %v", i, d.DecidedAt, original[i].DecidedAt)
		}
	}
}

func TestSaveDecisions_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()

	// Save initial decisions
	initial := []Decision{
		{
			ProposalID: "prop-1",
			Action:     "accepted",
			Reason:     "original reason",
		},
		{
			ProposalID: "prop-2",
			Action:     "rejected",
			Reason:     "keep this",
		},
	}

	if err := SaveDecisions(tmpDir, initial); err != nil {
		t.Fatalf("SaveDecisions (initial) failed: %v", err)
	}

	// Save new decision for prop-1 (overwrite) plus new decision for prop-3
	updated := []Decision{
		{
			ProposalID: "prop-1",
			Action:     "rejected",
			Reason:     "changed mind",
		},
		{
			ProposalID: "prop-3",
			Action:     "accepted",
			Reason:     "new proposal",
		},
	}

	if err := SaveDecisions(tmpDir, updated); err != nil {
		t.Fatalf("SaveDecisions (updated) failed: %v", err)
	}

	// Load and verify
	loaded, err := LoadDecisions(tmpDir)
	if err != nil {
		t.Fatalf("LoadDecisions failed: %v", err)
	}

	if len(loaded) != 3 {
		t.Fatalf("expected 3 decisions, got %d", len(loaded))
	}

	// Find each proposal by ID
	decisions := make(map[string]Decision)
	for _, d := range loaded {
		decisions[d.ProposalID] = d
	}

	// Check prop-1 was overwritten
	prop1 := decisions["prop-1"]
	if prop1.ProposalID != "prop-1" {
		t.Fatal("prop-1 not found in loaded decisions")
	}
	if prop1.Reason != "changed mind" {
		t.Errorf("prop-1 not overwritten: Reason is %q, want %q", prop1.Reason, "changed mind")
	}
	if prop1.Action != "rejected" {
		t.Errorf("prop-1 not overwritten: Action is %q, want %q", prop1.Action, "rejected")
	}

	// Check prop-2 was preserved
	prop2 := decisions["prop-2"]
	if prop2.ProposalID != "prop-2" {
		t.Fatal("prop-2 not found in loaded decisions")
	}
	if prop2.Reason != "keep this" {
		t.Errorf("prop-2 not preserved: Reason is %q, want %q", prop2.Reason, "keep this")
	}

	// Check prop-3 was added
	prop3 := decisions["prop-3"]
	if prop3.ProposalID != "prop-3" {
		t.Fatal("prop-3 not found in loaded decisions")
	}
	if prop3.Reason != "new proposal" {
		t.Errorf("prop-3 not added: Reason is %q, want %q", prop3.Reason, "new proposal")
	}
}

func TestIsTerminalDecision_DismissedReturnsTrue(t *testing.T) {
	decision := Decision{
		ProposalID: "prop-1",
		Action:     "dismissed",
	}

	if !IsTerminalDecision(decision) {
		t.Error("IsTerminalDecision should return true for dismissed action")
	}
}

func TestIsTerminalDecision_AcceptedReturnsFalse(t *testing.T) {
	decision := Decision{
		ProposalID: "prop-1",
		Action:     "accepted",
	}

	if IsTerminalDecision(decision) {
		t.Error("IsTerminalDecision should return false for accepted action")
	}
}

func TestIsTerminalDecision_RejectedReturnsFalse(t *testing.T) {
	decision := Decision{
		ProposalID: "prop-1",
		Action:     "rejected",
	}

	if IsTerminalDecision(decision) {
		t.Error("IsTerminalDecision should return false for rejected action")
	}
}

func TestIsTerminalDecision_EmptyActionReturnsFalse(t *testing.T) {
	decision := Decision{
		ProposalID: "prop-1",
		Action:     "",
	}

	if IsTerminalDecision(decision) {
		t.Error("IsTerminalDecision should return false for empty action")
	}
}

func TestFindExistingDecision_MatchingDecision(t *testing.T) {
	decisions := []Decision{
		{
			ProposalID: "prop-1",
			Action:     "accepted",
			Reason:     "looks good",
		},
		{
			ProposalID: "prop-2",
			Action:     "rejected",
			Reason:     "needs work",
		},
		{
			ProposalID: "prop-3",
			Action:     "dismissed",
			Reason:     "duplicate",
		},
	}

	decision, found := FindExistingDecision("prop-2", decisions)

	if !found {
		t.Error("FindExistingDecision should return true when decision is found")
	}
	if decision.ProposalID != "prop-2" {
		t.Errorf("FindExistingDecision returned wrong decision: ProposalID is %q, want %q", decision.ProposalID, "prop-2")
	}
	if decision.Action != "rejected" {
		t.Errorf("FindExistingDecision returned wrong decision: Action is %q, want %q", decision.Action, "rejected")
	}
	if decision.Reason != "needs work" {
		t.Errorf("FindExistingDecision returned wrong decision: Reason is %q, want %q", decision.Reason, "needs work")
	}
}

func TestFindExistingDecision_NotFound(t *testing.T) {
	decisions := []Decision{
		{
			ProposalID: "prop-1",
			Action:     "accepted",
			Reason:     "looks good",
		},
		{
			ProposalID: "prop-2",
			Action:     "rejected",
			Reason:     "needs work",
		},
	}

	decision, found := FindExistingDecision("prop-999", decisions)

	if found {
		t.Error("FindExistingDecision should return false when decision is not found")
	}
	if decision.ProposalID != "" {
		t.Errorf("FindExistingDecision should return zero Decision when not found, got ProposalID %q", decision.ProposalID)
	}
}

func TestFindExistingDecision_EmptySlice(t *testing.T) {
	decisions := []Decision{}

	decision, found := FindExistingDecision("prop-1", decisions)

	if found {
		t.Error("FindExistingDecision should return false for empty slice")
	}
	if decision.ProposalID != "" {
		t.Errorf("FindExistingDecision should return zero Decision for empty slice, got ProposalID %q", decision.ProposalID)
	}
}

func TestValidateTerminalState_NoDecisionsReturnsNil(t *testing.T) {
	decisions := []Decision{}

	err := ValidateTerminalState("prop-1", decisions)

	if err != nil {
		t.Errorf("ValidateTerminalState should return nil when no decisions exist, got: %v", err)
	}
}

func TestValidateTerminalState_AcceptedDecisionReturnsNil(t *testing.T) {
	decisions := []Decision{
		{
			ProposalID: "prop-1",
			Action:     "accepted",
			Reason:     "looks good",
		},
	}

	err := ValidateTerminalState("prop-1", decisions)

	if err != nil {
		t.Errorf("ValidateTerminalState should return nil for accepted decision, got: %v", err)
	}
}

func TestValidateTerminalState_RejectedDecisionReturnsNil(t *testing.T) {
	decisions := []Decision{
		{
			ProposalID: "prop-1",
			Action:     "rejected",
			Reason:     "needs work",
		},
	}

	err := ValidateTerminalState("prop-1", decisions)

	if err != nil {
		t.Errorf("ValidateTerminalState should return nil for rejected decision, got: %v", err)
	}
}

func TestValidateTerminalState_DismissedDecisionReturnsError(t *testing.T) {
	decisions := []Decision{
		{
			ProposalID: "prop-1",
			Action:     "dismissed",
			Reason:     "duplicate",
		},
	}

	err := ValidateTerminalState("prop-1", decisions)

	if err == nil {
		t.Error("ValidateTerminalState should return error for dismissed decision")
	}
	if err != nil && err.Error() != `proposal "prop-1" cannot be re-decided: it has been dismissed` {
		t.Errorf("ValidateTerminalState returned unexpected error message: %v", err)
	}
}

func TestValidateTerminalState_DifferentProposalDismissedReturnsNil(t *testing.T) {
	decisions := []Decision{
		{
			ProposalID: "prop-2",
			Action:     "dismissed",
			Reason:     "duplicate",
		},
	}

	err := ValidateTerminalState("prop-1", decisions)

	if err != nil {
		t.Errorf("ValidateTerminalState should return nil when different proposal is dismissed, got: %v", err)
	}
}
