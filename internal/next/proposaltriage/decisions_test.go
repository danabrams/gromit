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
			ProposalID:       "prop-1",
			Action:           "accepted",
			Reason:           "looks good",
			ApprovedTitle:    "Title 1",
			ApprovedChange:   "Change 1",
			ApprovedRationale: "Rationale 1",
			DecidedAt:        time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
		},
		{
			ProposalID:       "prop-2",
			Action:           "rejected",
			Reason:           "needs work",
			DuplicateOf:      "prop-3",
			DecidedAt:        time.Date(2026, 3, 21, 11, 0, 0, 0, time.UTC),
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
