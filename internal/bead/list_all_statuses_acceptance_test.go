//go:build acceptance

package bead

import (
	"testing"
)

// TestListWithLabel_ReturnsClosedBeads verifies that ListWithLabel returns closed beads
// when the --all flag is present in the command.
//
// Expected failure: Current implementation does not pass --all flag, so only open beads are returned.
func TestListWithLabel_ReturnsClosedBeads(t *testing.T) {
	c := newIsolatedClient(t)

	testLabel := "spec:closed-beads-test"

	// Create and close a bead with the test label
	bead, err := c.Create("Task to be closed", 1, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create test bead: %v", err)
	}

	// Close the bead
	if err := c.Close(bead.ID); err != nil {
		t.Skipf("Cannot close test bead: %v", err)
	}

	// Create an open bead with the same label
	_, err = c.Create("Open task", 1, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create open test bead: %v", err)
	}

	// Call ListWithLabel - should return both open and closed beads
	beads, err := c.ListWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error = %v", err)
	}

	// Count open and closed beads
	var openCount, closedCount int
	for _, b := range beads {
		if b.Status == "closed" {
			closedCount++
		} else {
			openCount++
		}
	}

	// Verify we got both open and closed beads
	if openCount == 0 {
		t.Errorf("ListWithLabel(%q) returned no open beads, expected at least 1", testLabel)
	}
	if closedCount == 0 {
		t.Errorf("ListWithLabel(%q) returned no closed beads, expected at least 1 (without --all flag, bd list defaults to open only)", testLabel)
	}
}

// TestListWithLabel_ReturnsUnlimitedResults verifies that ListWithLabel returns all beads
// without a 50-result cap when --limit 0 is present.
//
// Expected failure: Current implementation does not pass --limit 0, so results are capped at 50.
func TestListWithLabel_ReturnsUnlimitedResults(t *testing.T) {
	c := newIsolatedClient(t)

	testLabel := "spec:unlimited-test"

	// Create 55 beads (more than default limit of 50)
	const beadCount = 55
	for i := 0; i < beadCount; i++ {
		_, err := c.Create("Test bead for unlimited results", 1, []string{testLabel}, []string{})
		if err != nil {
			t.Skipf("Cannot create test bead %d: %v", i, err)
		}
	}

	// Call ListWithLabel - should return all 55 beads, not just 50
	beads, err := c.ListWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error = %v", err)
	}

	// Verify we got all beads, not just the default 50 limit
	if len(beads) < beadCount {
		t.Errorf("ListWithLabel(%q) returned %d beads, expected %d (without --limit 0, bd list defaults to 50-result cap)", testLabel, len(beads), beadCount)
	}
}

// TestListWithLabel_CommandIncludesAllFlag verifies that the bd list command includes --all flag.
//
// Expected failure: Current test expectations don't verify --all flag presence.
func TestListWithLabel_CommandIncludesAllFlag(t *testing.T) {
	c := newIsolatedClient(t)

	testLabel := "spec:verify-all-flag"

	// Create and close a bead
	bead, err := c.Create("Task for flag verification", 1, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create test bead: %v", err)
	}
	if err := c.Close(bead.ID); err != nil {
		t.Skipf("Cannot close test bead: %v", err)
	}

	// Call ListWithLabel and verify closed bead is returned
	// This indirectly verifies that --all flag is being used
	beads, err := c.ListWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error = %v", err)
	}

	// If --all flag is present, we should get the closed bead
	foundClosed := false
	for _, b := range beads {
		if b.ID == bead.ID && b.Status == "closed" {
			foundClosed = true
			break
		}
	}

	if !foundClosed {
		t.Errorf("ListWithLabel(%q) did not return closed bead %s, indicating --all flag is not being passed to bd list", testLabel, bead.ID)
	}
}

// TestListWithLabel_CommandIncludesLimitZeroFlag verifies that the bd list command includes --limit 0 flag.
//
// Expected failure: Current test expectations don't verify --limit 0 flag presence.
func TestListWithLabel_CommandIncludesLimitZeroFlag(t *testing.T) {
	// This test verifies that when we create more than 50 beads (the default limit),
	// all of them are returned, which proves --limit 0 is being passed.

	c := newIsolatedClient(t)

	testLabel := "spec:verify-limit-flag"

	// Create exactly 51 beads to exceed the default 50 limit
	const minBeadCount = 51
	createdIDs := []string{}
	for i := 0; i < minBeadCount; i++ {
		bead, err := c.Create("Limit verification bead", 1, []string{testLabel}, []string{})
		if err != nil {
			t.Skipf("Cannot create test bead %d: %v", i, err)
		}
		createdIDs = append(createdIDs, bead.ID)
	}

	// Call ListWithLabel
	beads, err := c.ListWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error = %v", err)
	}

	// If --limit 0 is present, we should get all beads
	if len(beads) < minBeadCount {
		t.Errorf("ListWithLabel(%q) returned %d beads when %d were created, indicating --limit 0 is not being passed (bd list defaults to --limit 50)", testLabel, len(beads), minBeadCount)
	}

	// Verify all created IDs are in the result
	foundIDs := make(map[string]bool)
	for _, b := range beads {
		foundIDs[b.ID] = true
	}

	missingCount := 0
	for _, id := range createdIDs {
		if !foundIDs[id] {
			missingCount++
		}
	}

	if missingCount > 0 {
		t.Errorf("ListWithLabel(%q) missing %d beads from result set (proves result is capped without --limit 0)", testLabel, missingCount)
	}
}
