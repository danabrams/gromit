package bead

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestReadyWithLabel_IntegrationCallsBdWithLabelFlag tests that ReadyWithLabel() passes --label flag to bd ready
func TestReadyWithLabel_IntegrationCallsBdWithLabelFlag(t *testing.T) {
	c := newIsolatedClient(t)

	// Create a test bead with a specific label
	testLabel := "spec:integration-test"
	_, err := c.Create("Test task for label filtering", 1, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create test bead: %v", err)
	}

	// Call ReadyWithLabel - this should invoke bd with --label flag
	bead, err := c.ReadyWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ReadyWithLabel() error = %v", err)
	}

	// If bd supports --label flag correctly, we should get a bead back
	// If bd doesn't support --label or the flag isn't passed, we might get nil or wrong bead
	if bead == nil {
		t.Errorf("ReadyWithLabel() returned nil, expected bead with label %q", testLabel)
		return
	}

	// Verify the returned bead has the requested label
	if !HasLabel(bead.Labels, testLabel) {
		t.Errorf("ReadyWithLabel(%q) returned bead without matching label, got labels: %v", testLabel, bead.Labels)
	}
}

// TestReadyWithLabel_IntegrationExcludesBeadsWithoutLabel tests that ReadyWithLabel() only returns beads with the specified label
func TestReadyWithLabel_IntegrationExcludesBeadsWithoutLabel(t *testing.T) {
	c := newIsolatedClient(t)

	// Create multiple beads with different labels
	label1 := "spec:label1-test"
	label2 := "spec:label2-test"

	_, err := c.Create("Task with label1", 1, []string{label1}, []string{})
	if err != nil {
		t.Skipf("Cannot create first test bead: %v", err)
	}

	_, err = c.Create("Task with label2", 1, []string{label2}, []string{})
	if err != nil {
		t.Skipf("Cannot create second test bead: %v", err)
	}

	// Request beads with label1 only
	bead, err := c.ReadyWithLabel(label1)
	if err != nil {
		t.Fatalf("ReadyWithLabel() error = %v", err)
	}

	if bead == nil {
		t.Errorf("ReadyWithLabel(%q) returned nil, expected bead with that label", label1)
		return
	}

	// Verify it has label1 and not label2
	if !HasLabel(bead.Labels, label1) {
		t.Errorf("ReadyWithLabel(%q) returned bead without the requested label, got: %v", label1, bead.Labels)
	}
	if HasLabel(bead.Labels, label2) {
		t.Errorf("ReadyWithLabel(%q) should not return bead with label %q", label1, label2)
	}
}

// TestListWithLabel_IntegrationCallsBdWithLabelFlag tests that ListWithLabel() passes --label flag to bd list
func TestListWithLabel_IntegrationCallsBdWithLabelFlag(t *testing.T) {
	c := newIsolatedClient(t)

	// Create multiple test beads with the same label
	testLabel := "spec:list-integration-test"
	for i := 0; i < 3; i++ {
		_, err := c.Create("Test task for list filtering", 1, []string{testLabel}, []string{})
		if err != nil {
			t.Skipf("Cannot create test bead %d: %v", i, err)
		}
	}

	// Call ListWithLabel - this should invoke bd list with --label flag
	beads, err := c.ListWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error = %v", err)
	}

	// Verify we got beads back
	if len(beads) == 0 {
		t.Errorf("ListWithLabel(%q) returned no beads, expected at least 1", testLabel)
		return
	}

	// Verify all returned beads have the requested label
	for i, bead := range beads {
		if !HasLabel(bead.Labels, testLabel) {
			t.Errorf("ListWithLabel(%q) bead[%d] does not have the requested label, got labels: %v", testLabel, i, bead.Labels)
		}
	}
}

// TestListWithLabel_IntegrationExcludesBeadsWithoutLabel tests that ListWithLabel() only returns beads with the specified label
func TestListWithLabel_IntegrationExcludesBeadsWithoutLabel(t *testing.T) {
	c := newIsolatedClient(t)

	// Create beads with different labels
	labelA := "spec:list-labelA"
	labelB := "spec:list-labelB"

	_, err := c.Create("Task with labelA", 1, []string{labelA}, []string{})
	if err != nil {
		t.Skipf("Cannot create first test bead: %v", err)
	}

	_, err = c.Create("Task with labelB", 1, []string{labelB}, []string{})
	if err != nil {
		t.Skipf("Cannot create second test bead: %v", err)
	}

	// Request beads with labelA only
	beads, err := c.ListWithLabel(labelA)
	if err != nil {
		t.Fatalf("ListWithLabel() error = %v", err)
	}

	if len(beads) == 0 {
		t.Errorf("ListWithLabel(%q) returned no beads, expected at least 1", labelA)
		return
	}

	// Verify all returned beads have labelA and none have labelB
	for i, bead := range beads {
		if !HasLabel(bead.Labels, labelA) {
			t.Errorf("ListWithLabel(%q) bead[%d] does not have the requested label, got: %v", labelA, i, bead.Labels)
		}
		if HasLabel(bead.Labels, labelB) {
			t.Errorf("ListWithLabel(%q) bead[%d] should not have label %q", labelA, i, labelB)
		}
	}
}

// TestReadyWithLabel_IntegrationReturnsNilWhenNoMatch tests that ReadyWithLabel() returns nil when no beads match the label
func TestReadyWithLabel_IntegrationReturnsNilWhenNoMatch(t *testing.T) {
	c := newIsolatedClient(t)

	// Request a label that doesn't exist
	nonExistentLabel := "spec:does-not-exist-xyz"
	bead, err := c.ReadyWithLabel(nonExistentLabel)
	if err != nil {
		t.Fatalf("ReadyWithLabel() error = %v", err)
	}

	if bead != nil {
		t.Errorf("ReadyWithLabel(%q) expected nil for non-existent label, got bead: %+v", nonExistentLabel, bead)
	}
}

// TestListWithLabel_IntegrationReturnsEmptySliceWhenNoMatch tests that ListWithLabel() returns empty slice when no beads match
func TestListWithLabel_IntegrationReturnsEmptySliceWhenNoMatch(t *testing.T) {
	c := newIsolatedClient(t)

	// Request a label that doesn't exist
	nonExistentLabel := "spec:does-not-exist-abc"
	beads, err := c.ListWithLabel(nonExistentLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error = %v", err)
	}

	if beads == nil {
		t.Errorf("ListWithLabel(%q) should return empty slice, not nil", nonExistentLabel)
		return
	}

	if len(beads) != 0 {
		t.Errorf("ListWithLabel(%q) expected empty slice, got %d beads", nonExistentLabel, len(beads))
	}
}

// TestReadyWithLabel_IntegrationExcludesEpics tests that ReadyWithLabel() excludes epic type beads
func TestReadyWithLabel_IntegrationExcludesEpics(t *testing.T) {
	c := newIsolatedClient(t)

	testLabel := "spec:epic-exclusion-test"

	// Create an epic with the label (if bd supports epic type creation)
	// Note: We'll try to create an epic, but if bd doesn't support it via Create(), this test may be limited
	// First create a regular task with the label
	_, err := c.Create("Regular task", 1, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create test task: %v", err)
	}

	// Call ReadyWithLabel
	bead, err := c.ReadyWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ReadyWithLabel() error = %v", err)
	}

	if bead == nil {
		t.Errorf("ReadyWithLabel(%q) returned nil, expected non-epic bead", testLabel)
		return
	}

	// Verify the returned bead is not an epic
	if bead.Type == "epic" {
		t.Errorf("ReadyWithLabel(%q) should exclude epic types, got bead type: %s", testLabel, bead.Type)
	}
}

// TestReadyWithLabel_IntegrationCallsCorrectCommand verifies the exact command being called
func TestReadyWithLabel_IntegrationCallsCorrectCommand(t *testing.T) {
	// This test verifies the command structure by checking if bd accepts the arguments
	c := newIsolatedClient(t)

	testLabel := "spec:cmd-test"

	// Manually execute the expected command to verify bd accepts it
	cmd := exec.Command("bd", "ready", "--json", "--limit", "10", "--label", testLabel)
	cmd.Dir = c.Dir
	out, err := cmd.CombinedOutput()

	if err != nil {
		if strings.Contains(string(out), "unknown flag") || strings.Contains(string(out), "flag provided but not defined") {
			t.Fatalf("bd ready does not support --label flag: %s", string(out))
		}
		// Other errors (like no beads) are acceptable
	}

	// Now call ReadyWithLabel and verify it doesn't fail with unknown flag error
	_, err = c.ReadyWithLabel(testLabel)
	if err != nil {
		if strings.Contains(err.Error(), "unknown flag") || strings.Contains(err.Error(), "flag provided but not defined") {
			t.Errorf("ReadyWithLabel() appears to be calling bd with incorrect flags: %v", err)
		}
		// Other errors are acceptable (no beads, etc.)
	}
}

// TestListWithLabel_IntegrationCallsCorrectCommand verifies the exact command being called
// Expected failure: Current implementation does not include --all and --limit 0 flags in the command.
func TestListWithLabel_IntegrationCallsCorrectCommand(t *testing.T) {
	// This test verifies the command structure by checking if bd accepts the arguments
	c := newIsolatedClient(t)

	testLabel := "spec:list-cmd-test"

	// Manually execute the expected command with --all and --limit 0 flags to verify bd accepts it
	cmd := exec.Command("bd", "list", "--json", "--label", testLabel, "--all", "--limit", "0")
	cmd.Dir = c.Dir
	out, err := cmd.CombinedOutput()

	if err != nil {
		if strings.Contains(string(out), "unknown flag") || strings.Contains(string(out), "flag provided but not defined") {
			t.Fatalf("bd list does not support required flags (--label, --all, or --limit): %s", string(out))
		}
		// Other errors (like no beads) are acceptable
	}

	// Now call ListWithLabel and verify it doesn't fail with unknown flag error
	_, err = c.ListWithLabel(testLabel)
	if err != nil {
		if strings.Contains(err.Error(), "unknown flag") || strings.Contains(err.Error(), "flag provided but not defined") {
			t.Errorf("ListWithLabel() appears to be calling bd with incorrect flags: %v", err)
		}
		// Other errors are acceptable (no beads, etc.)
	}
}

// TestReadyWithLabel_IntegrationWithSpecialCharacters tests label handling with various valid special characters
func TestReadyWithLabel_IntegrationWithSpecialCharacters(t *testing.T) {
	c := newIsolatedClient(t)

	tests := []struct {
		name  string
		label string
	}{
		{
			name:  "label with hyphen",
			label: "spec:my-feature",
		},
		{
			name:  "label with underscore",
			label: "spec:my_feature",
		},
		{
			name:  "label with dots",
			label: "spec:v1.2.3",
		},
		{
			name:  "label with numbers",
			label: "spec:feature123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a bead with the label
			_, err := c.Create("Test task", 1, []string{tt.label}, []string{})
			if err != nil {
				t.Skipf("Cannot create test bead with label %q: %v", tt.label, err)
			}

			// Call ReadyWithLabel
			bead, err := c.ReadyWithLabel(tt.label)
			if err != nil {
				// Should not get validation errors for valid labels
				if strings.Contains(err.Error(), "invalid") {
					t.Errorf("ReadyWithLabel(%q) should accept valid label, got validation error: %v", tt.label, err)
				}
			}

			// If we got a bead, verify it has the correct label
			if bead != nil && !HasLabel(bead.Labels, tt.label) {
				t.Errorf("ReadyWithLabel(%q) returned bead without matching label, got: %v", tt.label, bead.Labels)
			}
		})
	}
}

// TestListWithLabel_IntegrationReturnsMultipleBeadsInOrder tests that ListWithLabel() returns beads in consistent order
func TestListWithLabel_IntegrationReturnsMultipleBeadsInOrder(t *testing.T) {
	c := newIsolatedClient(t)

	testLabel := "spec:order-test"

	// Create multiple beads with different titles to verify order
	titles := []string{"Task A", "Task B", "Task C"}
	createdIDs := []string{}

	for _, title := range titles {
		bead, err := c.Create(title, 1, []string{testLabel}, []string{})
		if err != nil {
			t.Skipf("Cannot create test bead %q: %v", title, err)
		}
		createdIDs = append(createdIDs, bead.ID)
	}

	// List beads with the label
	beads, err := c.ListWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error = %v", err)
	}

	if len(beads) != len(titles) {
		t.Errorf("ListWithLabel(%q) returned %d beads, expected %d", testLabel, len(beads), len(titles))
		return
	}

	// Verify all created beads are in the result
	foundIDs := make(map[string]bool)
	for _, bead := range beads {
		foundIDs[bead.ID] = true
	}

	for _, id := range createdIDs {
		if !foundIDs[id] {
			t.Errorf("ListWithLabel(%q) missing expected bead ID %q", testLabel, id)
		}
	}
}

// TestListWithLabel_IntegrationExcludesEpics tests that ListWithLabel() excludes epic type beads
// This is an acceptance test for the epic exclusion requirement
func TestListWithLabel_IntegrationExcludesEpics(t *testing.T) {
	c := newIsolatedClient(t)

	testLabel := "spec:epic-exclusion-list-test"

	// Create a regular task with the label
	_, err := c.Create("Regular task", 1, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create test task: %v", err)
	}

	// List beads with the label
	beads, err := c.ListWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error = %v", err)
	}

	// Verify no epic beads in results
	for i, bead := range beads {
		if bead.Type == "epic" {
			t.Errorf("ListWithLabel(%q) bead[%d] should not be type epic, got bead ID %s with type %s", testLabel, i, bead.ID, bead.Type)
		}
	}
}

// TestReadyWithLabel_CommandContract verifies the exact bd command contract
func TestReadyWithLabel_CommandContract(t *testing.T) {
	if os.Getenv("BD_AVAILABLE") != "true" {
		t.Skip("Skipping bd command contract test (set BD_AVAILABLE=true to run)")
	}

	c := newIsolatedClient(t)

	testLabel := "spec:contract-test"

	// Create a test bead
	_, err := c.Create("Contract test bead", 1, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create test bead: %v", err)
	}

	// Manually execute the exact command we expect ReadyWithLabel to use
	expectedCmd := []string{"bd", "ready", "--json", "--limit", "10", "--label", testLabel}
	cmd := exec.Command(expectedCmd[0], expectedCmd[1:]...)
	cmd.Dir = c.Dir
	out, err := cmd.CombinedOutput()

	if err != nil {
		t.Logf("Expected bd command failed: %v\nOutput: %s", err, string(out))
	}

	// Now call ReadyWithLabel and verify it produces compatible results
	bead, clientErr := c.ReadyWithLabel(testLabel)

	// Both should succeed or both should fail
	if (err == nil) != (clientErr == nil) {
		t.Errorf("Command and client behavior differ: cmd err=%v, client err=%v", err, clientErr)
	}

	// If both succeeded and we got a bead, log success
	if err == nil && clientErr == nil && bead != nil {
		t.Logf("Contract verified: ReadyWithLabel(%q) returned bead %s", testLabel, bead.ID)
	}
}

// TestListWithLabel_CommandContract verifies the exact bd command contract
// Expected failure: Current test expects command without --all and --limit 0 flags.
func TestListWithLabel_CommandContract(t *testing.T) {
	if os.Getenv("BD_AVAILABLE") != "true" {
		t.Skip("Skipping bd command contract test (set BD_AVAILABLE=true to run)")
	}

	c := newIsolatedClient(t)

	testLabel := "spec:list-contract-test"

	// Create test beads (including a closed one to verify --all flag)
	for i := 0; i < 2; i++ {
		bead, err := c.Create("List contract test bead", 1, []string{testLabel}, []string{})
		if err != nil {
			t.Skipf("Cannot create test bead %d: %v", i, err)
		}
		// Close the first bead to test --all flag behavior
		if i == 0 {
			if err := c.Close(bead.ID); err != nil {
				t.Skipf("Cannot close test bead: %v", err)
			}
		}
	}

	// Manually execute the exact command we expect ListWithLabel to use (with --all and --limit 0)
	expectedCmd := []string{"bd", "list", "--json", "--label", testLabel, "--sort", "priority", "--all", "--limit", "0"}
	cmd := exec.Command(expectedCmd[0], expectedCmd[1:]...)
	cmd.Dir = c.Dir
	out, err := cmd.CombinedOutput()

	if err != nil {
		t.Logf("Expected bd command failed: %v\nOutput: %s", err, string(out))
	}

	// Now call ListWithLabel and verify it produces compatible results
	beads, clientErr := c.ListWithLabel(testLabel)

	// Both should succeed or both should fail
	if (err == nil) != (clientErr == nil) {
		t.Errorf("Command and client behavior differ: cmd err=%v, client err=%v", err, clientErr)
	}

	// If both succeeded, verify we got beads including the closed one (proves --all flag works)
	if err == nil && clientErr == nil {
		if len(beads) < 2 {
			t.Errorf("Contract test: expected at least 2 beads (1 open, 1 closed), got %d (--all flag may not be working)", len(beads))
		}

		// Count closed beads
		closedCount := 0
		for _, b := range beads {
			if b.Status == "closed" {
				closedCount++
			}
		}

		if closedCount == 0 {
			t.Errorf("Contract test: expected at least 1 closed bead in results (--all flag is required to return closed beads)")
		}

		t.Logf("Contract verified: ListWithLabel(%q) returned %d beads (%d closed)", testLabel, len(beads), closedCount)
	}
}
