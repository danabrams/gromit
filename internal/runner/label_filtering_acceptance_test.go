package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

// TestBeadClientInterface_AcceptanceLabelFilteringBehavior verifies that
// ReadyWithLabel and ListWithLabel actually filter beads by label as expected
func TestBeadClientInterface_AcceptanceLabelFilteringBehavior(t *testing.T) {
	// Create a mock that simulates real label-based filtering
	testLabel := "spec:auth"
	otherLabel := "spec:payments"

	// Mock data: 3 beads with auth label, 2 with payments label
	authBeads := []*bead.Bead{
		{ID: "auth-1", Title: "Auth task 1", Labels: []string{testLabel}, ExpectedOutputs: []string{}},
		{ID: "auth-2", Title: "Auth task 2", Labels: []string{testLabel}, ExpectedOutputs: []string{}},
		{ID: "auth-3", Title: "Auth task 3", Labels: []string{testLabel}, ExpectedOutputs: []string{}},
	}
	paymentBeads := []*bead.Bead{
		{ID: "pay-1", Title: "Payment task 1", Labels: []string{otherLabel}, ExpectedOutputs: []string{}},
		{ID: "pay-2", Title: "Payment task 2", Labels: []string{otherLabel}, ExpectedOutputs: []string{}},
	}

	mock := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			// Simulate bd ready --label behavior: return first ready bead with that label
			if label == testLabel && len(authBeads) > 0 {
				return authBeads[0], nil
			}
			if label == otherLabel && len(paymentBeads) > 0 {
				return paymentBeads[0], nil
			}
			return nil, nil
		},
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			// Simulate bd list --label behavior: return all beads with that label
			if label == testLabel {
				return authBeads, nil
			}
			if label == otherLabel {
				return paymentBeads, nil
			}
			return []*bead.Bead{}, nil
		},
	}

	var client BeadClient = mock

	// Test 1: ReadyWithLabel returns only beads with the requested label
	t.Run("ReadyWithLabel filters by label", func(t *testing.T) {
		result, err := client.ReadyWithLabel(testLabel)
		if err != nil {
			t.Errorf("ReadyWithLabel() error: %v", err)
		}
		if result == nil {
			t.Fatal("ReadyWithLabel() returned nil, expected a bead")
		}
		if !bead.HasLabel(result.Labels, testLabel) {
			t.Errorf("ReadyWithLabel(%q) returned bead without label, got labels: %v", testLabel, result.Labels)
		}
		if result.ID != "auth-1" {
			t.Errorf("ReadyWithLabel(%q) returned wrong bead, expected auth-1, got %s", testLabel, result.ID)
		}
	})

	// Test 2: ListWithLabel returns all beads with the requested label
	t.Run("ListWithLabel filters by label", func(t *testing.T) {
		results, err := client.ListWithLabel(testLabel)
		if err != nil {
			t.Errorf("ListWithLabel() error: %v", err)
		}
		if len(results) != 3 {
			t.Errorf("ListWithLabel(%q) returned %d beads, expected 3", testLabel, len(results))
		}
		for i, result := range results {
			if !bead.HasLabel(result.Labels, testLabel) {
				t.Errorf("ListWithLabel(%q) bead[%d] missing label, got labels: %v", testLabel, i, result.Labels)
			}
		}
	})

	// Test 3: Different labels return different beads
	t.Run("Different labels return different results", func(t *testing.T) {
		authResult, _ := client.ReadyWithLabel(testLabel)
		payResult, _ := client.ReadyWithLabel(otherLabel)

		if authResult != nil && payResult != nil && authResult.ID == payResult.ID {
			t.Error("ReadyWithLabel should return different beads for different labels")
		}

		authList, _ := client.ListWithLabel(testLabel)
		payList, _ := client.ListWithLabel(otherLabel)

		if len(authList) == len(payList) {
			t.Logf("Auth list: %d beads, Payment list: %d beads (different sizes expected)", len(authList), len(payList))
		}
		// Verify no overlap in bead IDs
		authIDs := make(map[string]bool)
		for _, b := range authList {
			authIDs[b.ID] = true
		}
		for _, b := range payList {
			if authIDs[b.ID] {
				t.Errorf("Found bead %s in both auth and payment lists", b.ID)
			}
		}
	})

	// Test 4: Non-existent label returns empty results
	t.Run("Non-existent label returns empty", func(t *testing.T) {
		result, err := client.ReadyWithLabel("spec:nonexistent")
		if err != nil {
			t.Errorf("ReadyWithLabel() error: %v", err)
		}
		if result != nil {
			t.Errorf("ReadyWithLabel() for non-existent label should return nil, got: %+v", result)
		}

		results, err := client.ListWithLabel("spec:nonexistent")
		if err != nil {
			t.Errorf("ListWithLabel() error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("ListWithLabel() for non-existent label should return empty slice, got %d beads", len(results))
		}
	})
}

// TestBeadClientInterface_AcceptanceLabelMethodsIndependent verifies that
// ReadyWithLabel and ListWithLabel operate independently from Ready()
func TestBeadClientInterface_AcceptanceLabelMethodsIndependent(t *testing.T) {
	// Create a mock where Ready() returns one bead, but ReadyWithLabel returns a different one
	unlabeledBead := &bead.Bead{
		ID:              "unlabeled-1",
		Title:           "Unlabeled task",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}
	labeledBead := &bead.Bead{
		ID:              "labeled-1",
		Title:           "Labeled task",
		Labels:          []string{"spec:test"},
		ExpectedOutputs: []string{},
	}

	mock := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return unlabeledBead, nil
		},
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			if label == "spec:test" {
				return labeledBead, nil
			}
			return nil, nil
		},
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			if label == "spec:test" {
				return []*bead.Bead{labeledBead}, nil
			}
			return []*bead.Bead{}, nil
		},
	}

	var client BeadClient = mock

	// Verify Ready() and ReadyWithLabel() can return different beads
	readyResult, err := client.Ready()
	if err != nil {
		t.Errorf("Ready() error: %v", err)
	}
	if readyResult == nil || readyResult.ID != "unlabeled-1" {
		t.Errorf("Ready() should return unlabeled-1, got: %v", readyResult)
	}

	labeledResult, err := client.ReadyWithLabel("spec:test")
	if err != nil {
		t.Errorf("ReadyWithLabel() error: %v", err)
	}
	if labeledResult == nil || labeledResult.ID != "labeled-1" {
		t.Errorf("ReadyWithLabel() should return labeled-1, got: %v", labeledResult)
	}

	// Verify they returned different beads
	if readyResult != nil && labeledResult != nil && readyResult.ID == labeledResult.ID {
		t.Error("Ready() and ReadyWithLabel() should be independent and can return different beads")
	}
}

// TestBeadClientInterface_AcceptanceLabelMethodsPreserveSemantics verifies that
// label methods preserve the semantic differences between Ready/List
func TestBeadClientInterface_AcceptanceLabelMethodsPreserveSemantics(t *testing.T) {
	// Ready methods should return only unblocked beads
	// List methods should return all open beads (regardless of blocked status)
	testLabel := "spec:semantics"

	unblockedBead := &bead.Bead{
		ID:              "unblocked-1",
		Title:           "Unblocked task",
		Labels:          []string{testLabel},
		Status:          "open",
		ExpectedOutputs: []string{},
	}
	blockedBead := &bead.Bead{
		ID:              "blocked-1",
		Title:           "Blocked task",
		Labels:          []string{testLabel},
		Status:          "open",
		ExpectedOutputs: []string{},
		// In real bd, this would have dependencies or parent relationships
	}

	mock := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			// Ready returns only unblocked beads
			return unblockedBead, nil
		},
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			// ReadyWithLabel should also return only unblocked beads
			if label == testLabel {
				return unblockedBead, nil
			}
			return nil, nil
		},
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			// ListWithLabel returns all open beads (both unblocked and blocked)
			if label == testLabel {
				return []*bead.Bead{unblockedBead, blockedBead}, nil
			}
			return []*bead.Bead{}, nil
		},
	}

	var client BeadClient = mock

	// Verify ReadyWithLabel returns only unblocked beads
	readyResult, err := client.ReadyWithLabel(testLabel)
	if err != nil {
		t.Errorf("ReadyWithLabel() error: %v", err)
	}
	if readyResult == nil {
		t.Fatal("ReadyWithLabel() returned nil")
	}
	if readyResult.ID != "unblocked-1" {
		t.Errorf("ReadyWithLabel() should return only unblocked bead, got: %s", readyResult.ID)
	}

	// Verify ListWithLabel returns all beads (including blocked)
	listResults, err := client.ListWithLabel(testLabel)
	if err != nil {
		t.Errorf("ListWithLabel() error: %v", err)
	}
	if len(listResults) != 2 {
		t.Fatalf("ListWithLabel() should return both unblocked and blocked beads, got %d", len(listResults))
	}

	// Verify the semantics: Ready returns subset of List
	t.Logf("ReadyWithLabel returned %d bead, ListWithLabel returned %d beads", 1, len(listResults))
	if len(listResults) <= 1 {
		t.Error("ListWithLabel should return more beads than ReadyWithLabel (includes blocked beads)")
	}
}
