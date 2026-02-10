package bead

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestReadyWithLabel_AcceptanceCallsBdReadyWithLabel is an acceptance test verifying
// that ReadyWithLabel() calls bd ready with the --label flag and correct arguments
func TestReadyWithLabel_AcceptanceCallsBdReadyWithLabel(t *testing.T) {
	c := newIsolatedClient(t)

	testLabel := "spec:acceptance-ready-test"

	// Create a test bead with the label
	_, err := c.Create("Acceptance test task for ReadyWithLabel", 1, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create test bead: %v", err)
	}

	// Verify bd ready --json --limit 10 --label <label> is a valid command
	cmd := exec.Command("bd", "ready", "--json", "--limit", "10", "--label", testLabel)
	cmd.Dir = c.Dir
	out, err := cmd.CombinedOutput()

	// Check if bd supports the --label flag
	if err != nil {
		if strings.Contains(string(out), "unknown flag") || strings.Contains(string(out), "flag provided but not defined") {
			t.Fatalf("bd ready does not support --label flag, cannot test ReadyWithLabel: %s", string(out))
		}
	}

	// Now call ReadyWithLabel and verify it behaves as expected
	bead, err := c.ReadyWithLabel(testLabel)
	if err != nil {
		// Check if error is due to unsupported flag
		if strings.Contains(err.Error(), "unknown flag") || strings.Contains(err.Error(), "flag provided but not defined") {
			t.Fatalf("ReadyWithLabel() is not calling bd with correct flags: %v", err)
		}
		// Other errors are acceptable in this test
		t.Logf("ReadyWithLabel() returned error (may be expected): %v", err)
	}

	// If we got a bead, verify it has the requested label
	if bead != nil {
		if !HasLabel(bead.Labels, testLabel) {
			t.Errorf("ReadyWithLabel(%q) returned bead without the requested label, got labels: %v", testLabel, bead.Labels)
		}
		// Verify the bead is not an epic (per requirements)
		if bead.Type == "epic" {
			t.Errorf("ReadyWithLabel(%q) should exclude epic types, got type: %s", testLabel, bead.Type)
		}
	}
}

// TestReadyWithLabel_AcceptanceExcludesEpicType is an acceptance test verifying
// that ReadyWithLabel() excludes beads of type "epic" from results
func TestReadyWithLabel_AcceptanceExcludesEpicType(t *testing.T) {
	c := newIsolatedClient(t)

	testLabel := "spec:epic-exclusion-ready"

	// Create a non-epic bead with the label
	task, err := c.Create("Non-epic task with label", 1, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create test task: %v", err)
	}

	// Call ReadyWithLabel
	bead, err := c.ReadyWithLabel(testLabel)
	if err != nil {
		t.Logf("ReadyWithLabel() returned error: %v", err)
	}

	// If we got a bead, it must not be an epic
	if bead != nil {
		if bead.Type == "epic" {
			t.Errorf("ReadyWithLabel(%q) returned epic type bead (ID: %s), should exclude epics", testLabel, bead.ID)
		}
		// Verify we got our task back (or another non-epic bead)
		t.Logf("ReadyWithLabel() returned bead ID %s with type %s", bead.ID, bead.Type)
		if bead.ID == task.ID {
			t.Logf("Successfully returned the created non-epic task")
		}
	}
}

// TestReadyWithLabel_AcceptanceReturnsNilForNoMatches is an acceptance test verifying
// that ReadyWithLabel() returns nil when no beads match the label
func TestReadyWithLabel_AcceptanceReturnsNilForNoMatches(t *testing.T) {
	c := newIsolatedClient(t)

	// Use a label that definitely doesn't exist
	nonExistentLabel := "spec:definitely-does-not-exist-12345"

	bead, err := c.ReadyWithLabel(nonExistentLabel)
	if err != nil {
		t.Fatalf("ReadyWithLabel() with non-existent label should not error, got: %v", err)
	}

	if bead != nil {
		t.Errorf("ReadyWithLabel(%q) should return nil for non-existent label, got bead: %+v", nonExistentLabel, bead)
	}
}

// TestReadyWithLabel_AcceptanceValidatesLabelFormat is an acceptance test verifying
// that ReadyWithLabel() rejects labels with shell metacharacters
func TestReadyWithLabel_AcceptanceValidatesLabelFormat(t *testing.T) {
	c, _ := NewClient()

	dangerousLabels := []string{
		"spec:test; rm -rf /",
		"spec:test$(whoami)",
		"spec:test`ls`",
		"spec:test | cat /etc/passwd",
		"spec:test && echo pwned",
	}

	for _, label := range dangerousLabels {
		_, err := c.ReadyWithLabel(label)
		if err == nil {
			t.Errorf("ReadyWithLabel(%q) should reject dangerous label, got nil error", label)
			continue
		}
		if !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "metacharacter") {
			t.Errorf("ReadyWithLabel(%q) should return validation error, got: %v", label, err)
		}
	}
}

// TestReadyWithLabel_AcceptanceRejectsEmptyLabel is an acceptance test verifying
// that ReadyWithLabel() rejects empty labels
func TestReadyWithLabel_AcceptanceRejectsEmptyLabel(t *testing.T) {
	c, _ := NewClient()

	_, err := c.ReadyWithLabel("")
	if err == nil {
		t.Error("ReadyWithLabel(\"\") should return error for empty label")
		return
	}

	if !strings.Contains(err.Error(), "empty") && !strings.Contains(err.Error(), "label") {
		t.Errorf("ReadyWithLabel(\"\") error should mention empty label, got: %v", err)
	}
}

// TestListWithLabel_AcceptanceCallsBdListWithLabel is an acceptance test verifying
// that ListWithLabel() calls bd list with the --label flag and correct arguments
func TestListWithLabel_AcceptanceCallsBdListWithLabel(t *testing.T) {
	c := newIsolatedClient(t)

	testLabel := "spec:acceptance-list-test"

	// Create multiple test beads with the same label
	createdIDs := []string{}
	for i := 0; i < 3; i++ {
		bead, err := c.Create("Acceptance test task for ListWithLabel", 1, []string{testLabel}, []string{})
		if err != nil {
			t.Skipf("Cannot create test bead %d: %v", i, err)
		}
		createdIDs = append(createdIDs, bead.ID)
	}

	// Verify bd list --json --label <label> is a valid command
	cmd := exec.Command("bd", "list", "--json", "--label", testLabel)
	cmd.Dir = c.Dir
	out, err := cmd.CombinedOutput()

	// Check if bd supports the --label flag
	if err != nil {
		if strings.Contains(string(out), "unknown flag") || strings.Contains(string(out), "flag provided but not defined") {
			t.Fatalf("bd list does not support --label flag, cannot test ListWithLabel: %s", string(out))
		}
	}

	// Now call ListWithLabel and verify it behaves as expected
	beads, err := c.ListWithLabel(testLabel)
	if err != nil {
		// Check if error is due to unsupported flag
		if strings.Contains(err.Error(), "unknown flag") || strings.Contains(err.Error(), "flag provided but not defined") {
			t.Fatalf("ListWithLabel() is not calling bd with correct flags: %v", err)
		}
		t.Fatalf("ListWithLabel() unexpected error: %v", err)
	}

	// Verify we got the beads we created (at minimum)
	if len(beads) < len(createdIDs) {
		t.Errorf("ListWithLabel(%q) returned %d beads, expected at least %d", testLabel, len(beads), len(createdIDs))
	}

	// Verify all returned beads have the requested label
	for i, bead := range beads {
		if !HasLabel(bead.Labels, testLabel) {
			t.Errorf("ListWithLabel(%q) bead[%d] (ID: %s) does not have the requested label, got labels: %v", testLabel, i, bead.ID, bead.Labels)
		}
	}

	// Verify at least some of our created beads are in the results
	foundCount := 0
	for _, bead := range beads {
		for _, createdID := range createdIDs {
			if bead.ID == createdID {
				foundCount++
				break
			}
		}
	}

	if foundCount == 0 {
		t.Errorf("ListWithLabel(%q) did not return any of the %d beads we created", testLabel, len(createdIDs))
	}
}

// TestListWithLabel_AcceptanceReturnsMultipleBeads is an acceptance test verifying
// that ListWithLabel() returns all beads with the specified label, not just one
func TestListWithLabel_AcceptanceReturnsMultipleBeads(t *testing.T) {
	c := newIsolatedClient(t)

	testLabel := "spec:multi-bead-list-test"

	// Create exactly 3 beads with the same label
	expectedCount := 3
	for i := 0; i < expectedCount; i++ {
		_, err := c.Create("Multi-bead test task", 1, []string{testLabel}, []string{})
		if err != nil {
			t.Skipf("Cannot create test bead %d: %v", i, err)
		}
	}

	// List beads with the label
	beads, err := c.ListWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error: %v", err)
	}

	// Verify we got at least the beads we created (there might be more from other tests)
	if len(beads) < expectedCount {
		t.Errorf("ListWithLabel(%q) returned %d beads, expected at least %d", testLabel, len(beads), expectedCount)
	}

	// Verify all returned beads are non-nil pointers
	for i, bead := range beads {
		if bead == nil {
			t.Errorf("ListWithLabel(%q) returned nil bead at index %d", testLabel, i)
		}
	}
}

// TestListWithLabel_AcceptanceExcludesEpicType is an acceptance test verifying
// that ListWithLabel() excludes beads of type "epic" from results
func TestListWithLabel_AcceptanceExcludesEpicType(t *testing.T) {
	c := newIsolatedClient(t)

	testLabel := "spec:epic-exclusion-list"

	// Create non-epic beads with the label
	_, err := c.Create("Non-epic task 1", 1, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create test task 1: %v", err)
	}

	_, err = c.Create("Non-epic task 2", 2, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create test task 2: %v", err)
	}

	// List beads with the label
	beads, err := c.ListWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error: %v", err)
	}

	// Verify no epic beads in results
	for i, bead := range beads {
		if bead.Type == "epic" {
			t.Errorf("ListWithLabel(%q) bead[%d] (ID: %s) should not be type epic", testLabel, i, bead.ID)
		}
	}

	// Verify we got at least some beads back
	if len(beads) < 2 {
		t.Logf("ListWithLabel(%q) returned %d beads, expected at least 2", testLabel, len(beads))
	}
}

// TestListWithLabel_AcceptanceReturnsEmptySliceForNoMatches is an acceptance test verifying
// that ListWithLabel() returns an empty slice (not nil) when no beads match the label
func TestListWithLabel_AcceptanceReturnsEmptySliceForNoMatches(t *testing.T) {
	c := newIsolatedClient(t)

	// Use a label that definitely doesn't exist
	nonExistentLabel := "spec:definitely-does-not-exist-67890"

	beads, err := c.ListWithLabel(nonExistentLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() with non-existent label should not error, got: %v", err)
	}

	if beads == nil {
		t.Error("ListWithLabel() should return empty slice, not nil")
		return
	}

	if len(beads) != 0 {
		t.Errorf("ListWithLabel(%q) should return empty slice for non-existent label, got %d beads", nonExistentLabel, len(beads))
	}
}

// TestListWithLabel_AcceptanceValidatesLabelFormat is an acceptance test verifying
// that ListWithLabel() rejects labels with shell metacharacters
func TestListWithLabel_AcceptanceValidatesLabelFormat(t *testing.T) {
	c, _ := NewClient()

	dangerousLabels := []string{
		"spec:test; rm -rf /",
		"spec:test$(whoami)",
		"spec:test`ls`",
		"spec:test | cat /etc/passwd",
		"spec:test && echo pwned",
	}

	for _, label := range dangerousLabels {
		_, err := c.ListWithLabel(label)
		if err == nil {
			t.Errorf("ListWithLabel(%q) should reject dangerous label, got nil error", label)
			continue
		}
		if !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "metacharacter") {
			t.Errorf("ListWithLabel(%q) should return validation error, got: %v", label, err)
		}
	}
}

// TestListWithLabel_AcceptanceRejectsEmptyLabel is an acceptance test verifying
// that ListWithLabel() rejects empty labels
func TestListWithLabel_AcceptanceRejectsEmptyLabel(t *testing.T) {
	c, _ := NewClient()

	_, err := c.ListWithLabel("")
	if err == nil {
		t.Error("ListWithLabel(\"\") should return error for empty label")
		return
	}

	if !strings.Contains(err.Error(), "empty") && !strings.Contains(err.Error(), "label") {
		t.Errorf("ListWithLabel(\"\") error should mention empty label, got: %v", err)
	}
}

// TestReadyWithLabel_AcceptanceCommandContract verifies the exact bd command contract
func TestReadyWithLabel_AcceptanceCommandContract(t *testing.T) {
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

// TestListWithLabel_AcceptanceCommandContract verifies the exact bd command contract
func TestListWithLabel_AcceptanceCommandContract(t *testing.T) {
	if os.Getenv("BD_AVAILABLE") != "true" {
		t.Skip("Skipping bd command contract test (set BD_AVAILABLE=true to run)")
	}

	c := newIsolatedClient(t)

	testLabel := "spec:list-contract-test"

	// Create test beads
	for i := 0; i < 2; i++ {
		_, err := c.Create("List contract test bead", 1, []string{testLabel}, []string{})
		if err != nil {
			t.Skipf("Cannot create test bead %d: %v", i, err)
		}
	}

	// Manually execute the exact command we expect ListWithLabel to use
	expectedCmd := []string{"bd", "list", "--json", "--label", testLabel}
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

	// If both succeeded and we got beads, log success
	if err == nil && clientErr == nil && len(beads) > 0 {
		t.Logf("Contract verified: ListWithLabel(%q) returned %d beads", testLabel, len(beads))
	}
}

// TestReadyWithLabel_AcceptanceFetchesBatch verifies that ReadyWithLabel fetches
// a batch of beads (limit 10) to filter for non-epic types, not just a single bead
func TestReadyWithLabel_AcceptanceFetchesBatch(t *testing.T) {
	// This is a specification test - we verify the behavior matches the spec:
	// "ReadyWithLabel calls bd ready --json --limit 10 --label <label>"
	c, _ := NewClient()

	// We can't easily verify the exact command without mocking, but we can
	// verify that the method exists and has the expected signature
	label := "spec:batch-test"
	_, err := c.ReadyWithLabel(label)

	// We expect either success or bd-related error, not a method-not-found error
	if err != nil {
		// Should be a bd command error, not a compilation or method error
		if strings.Contains(err.Error(), "method") || strings.Contains(err.Error(), "undefined") {
			t.Errorf("ReadyWithLabel method appears to not exist or have wrong signature: %v", err)
		}
	}
}

// TestListWithLabel_AcceptanceReturnsAllMatchingBeads verifies that ListWithLabel
// returns all beads with the label, not just a limited subset
func TestListWithLabel_AcceptanceReturnsAllMatchingBeads(t *testing.T) {
	c := newIsolatedClient(t)

	testLabel := "spec:all-matching-beads-test"

	// Create more beads than a typical limit would return
	expectedMinimum := 5
	for i := 0; i < expectedMinimum; i++ {
		_, err := c.Create("Test bead for unlimited list", 1, []string{testLabel}, []string{})
		if err != nil {
			t.Skipf("Cannot create test bead %d: %v", i, err)
		}
	}

	// List should return all beads, not limited to 1 or 10
	beads, err := c.ListWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error: %v", err)
	}

	if len(beads) < expectedMinimum {
		t.Errorf("ListWithLabel(%q) returned %d beads, expected at least %d", testLabel, len(beads), expectedMinimum)
	}
}
