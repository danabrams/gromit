//go:build acceptance

package bead

import (
	"testing"
)

// TestListWithLabel_IncludesBothOpenAndClosedBeads verifies that ListWithLabel returns both open and closed beads.
// This is a core acceptance criterion for the --all flag addition.
func TestListWithLabel_IncludesBothOpenAndClosedBeads(t *testing.T) {
	// Expected failure: ListWithLabel at internal/bead/bead.go:616 does not pass --all flag
	// Current command: c.run("list", "--json", "--label", label, "--sort", "priority")
	// Expected command: c.run("list", "--json", "--label", label, "--sort", "priority", "--all", "--limit", "0")
	//
	// Without --all, bd list defaults to showing only open beads, so closed beads are never returned.
	// This test verifies that when beads with the same label exist in both open and closed states,
	// ListWithLabel returns both.

	c := newIsolatedClient(t)

	testLabel := "spec:all-statuses-test"

	// Create an open bead with the test label
	openBead, err := c.Create("Open task", 1, []string{testLabel}, []string{"Verify open bead"})
	if err != nil {
		t.Skipf("Cannot create open bead: %v", err)
	}
	openID := openBead.ID

	// Create a second bead and close it
	closedBead, err := c.Create("Task to close", 1, []string{testLabel}, []string{"Verify closed bead"})
	if err != nil {
		t.Skipf("Cannot create bead to close: %v", err)
	}
	closedID := closedBead.ID

	// Close the second bead
	if err := c.Close(closedID); err != nil {
		t.Skipf("Cannot close bead: %v", err)
	}

	// Call ListWithLabel - should return both open and closed beads
	beads, err := c.ListWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error = %v", err)
	}

	// Map beads by ID for easy lookup
	beadsByID := make(map[string]*Bead)
	for _, b := range beads {
		beadsByID[b.ID] = b
	}

	// Verify the open bead is present
	if _, found := beadsByID[openID]; !found {
		t.Errorf("Expected to find open bead %s in results, but it was not returned", openID)
	}

	// Verify the closed bead is present
	// Expected failure: closed bead will not be in results because --all flag is missing
	if _, found := beadsByID[closedID]; !found {
		t.Errorf("Expected to find closed bead %s in results, but it was not returned. This indicates --all flag is missing.", closedID)
	}

	// Verify we have both open and closed statuses in the results
	hasOpen := false
	hasClosed := false
	for _, b := range beads {
		if b.ID == openID && b.Status == "open" {
			hasOpen = true
		}
		if b.ID == closedID && b.Status == "closed" {
			hasClosed = true
		}
	}

	if !hasOpen {
		t.Errorf("Expected to find at least one bead with status='open'")
	}
	if !hasClosed {
		t.Errorf("Expected to find at least one bead with status='closed'. Without --all flag, closed beads are filtered out by bd list.")
	}
}

// TestListWithLabel_ReturnsMoreThan50Beads verifies that ListWithLabel returns all beads without a 50-result cap.
// This is a core acceptance criterion for the --limit 0 flag addition.
func TestListWithLabel_ReturnsMoreThan50Beads(t *testing.T) {
	// Expected failure: ListWithLabel at internal/bead/bead.go:616 does not pass --limit 0
	// Current command: c.run("list", "--json", "--label", label, "--sort", "priority")
	// Expected command: c.run("list", "--json", "--label", label, "--sort", "priority", "--all", "--limit", "0")
	//
	// Without --limit 0, bd list defaults to a 50-result cap, silently truncating results.
	// This test creates 55 beads and verifies all are returned.

	c := newIsolatedClient(t)

	testLabel := "spec:no-limit-test"
	numBeads := 55

	// Create 55 beads with the same label
	createdIDs := make([]string, numBeads)
	for i := 0; i < numBeads; i++ {
		bead, err := c.Create("Task "+string(rune('A'+i%26)), 1, []string{testLabel}, []string{})
		if err != nil {
			t.Skipf("Cannot create bead %d: %v", i, err)
		}
		createdIDs[i] = bead.ID
	}

	// Call ListWithLabel - should return all 55 beads
	beads, err := c.ListWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error = %v", err)
	}

	// Verify we got all beads, not just the first 50
	// Expected failure: will get exactly 50 beads when --limit 0 is missing
	if len(beads) < numBeads {
		t.Errorf("ListWithLabel returned %d beads, expected at least %d. This indicates --limit 0 flag is missing (default limit is 50).", len(beads), numBeads)
	}

	// Verify all created beads are present
	beadsByID := make(map[string]bool)
	for _, b := range beads {
		beadsByID[b.ID] = true
	}

	missingCount := 0
	for _, id := range createdIDs {
		if !beadsByID[id] {
			missingCount++
		}
	}

	if missingCount > 0 {
		t.Errorf("ListWithLabel is missing %d beads out of %d created. Default 50-result limit is truncating results.", missingCount, numBeads)
	}

	// Specifically verify the result count matches exactly
	if len(beads) != numBeads {
		t.Errorf("ListWithLabel returned %d beads, expected exactly %d. Got %d beads, which suggests the default 50-result cap is active.", len(beads), numBeads, len(beads))
	}
}

// TestGetBeadCounts_ReportsNonZeroClosedCount verifies that getBeadCounts in epic.go
// correctly reports non-zero closed counts when closed beads exist with the queried label.
// This is the primary motivator for the fix - progress reporting was broken.
func TestGetBeadCounts_ReportsNonZeroClosedCount(t *testing.T) {
	// Expected failure: getBeadCounts will always report closed=0 because ListWithLabel
	// doesn't return closed beads without --all flag.
	//
	// getBeadCounts at cmd/gromit/epic.go:282 calls ListWithLabel and counts beads by status.
	// Without --all, ListWithLabel only returns open beads, so closed count is always 0.

	c := newIsolatedClient(t)

	testLabel := "spec:count-test"

	// Create 3 open beads
	for i := 0; i < 3; i++ {
		_, err := c.Create("Open task", 1, []string{testLabel}, []string{})
		if err != nil {
			t.Skipf("Cannot create open bead %d: %v", i, err)
		}
	}

	// Create 2 beads and close them
	for i := 0; i < 2; i++ {
		bead, err := c.Create("Task to close", 1, []string{testLabel}, []string{})
		if err != nil {
			t.Skipf("Cannot create bead to close %d: %v", i, err)
		}
		if err := c.Close(bead.ID); err != nil {
			t.Skipf("Cannot close bead: %v", err)
		}
	}

	// Call ListWithLabel and count by status (mimicking getBeadCounts behavior)
	beads, err := c.ListWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error = %v", err)
	}

	openCount := 0
	closedCount := 0
	for _, b := range beads {
		if b.Status == "closed" {
			closedCount++
		} else {
			openCount++
		}
	}

	// Verify open count is correct
	if openCount != 3 {
		t.Errorf("Expected 3 open beads, got %d", openCount)
	}

	// Verify closed count is non-zero
	// Expected failure: closedCount will be 0 because --all flag is missing
	if closedCount == 0 {
		t.Errorf("Expected 2 closed beads, got 0. This indicates ListWithLabel is not returning closed beads (missing --all flag).")
	}

	if closedCount != 2 {
		t.Errorf("Expected 2 closed beads, got %d", closedCount)
	}
}

// TestListWithLabel_StatusMixedResults verifies that ListWithLabel returns beads with
// various statuses when --all flag is present, not just open beads.
func TestListWithLabel_StatusMixedResults(t *testing.T) {
	// Expected failure: Only open beads will be returned without --all flag.
	//
	// This test creates beads with the same label across different lifecycle stages
	// and verifies that ListWithLabel returns all of them, regardless of status.

	c := newIsolatedClient(t)

	testLabel := "spec:mixed-status-test"

	// Create several beads with different eventual statuses
	bead1, err := c.Create("First task", 1, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create first bead: %v", err)
	}
	id1 := bead1.ID // This one stays open

	bead2, err := c.Create("Second task", 1, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create second bead: %v", err)
	}
	id2 := bead2.ID
	// Close the second one
	if err := c.Close(id2); err != nil {
		t.Skipf("Cannot close second bead: %v", err)
	}

	bead3, err := c.Create("Third task", 1, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create third bead: %v", err)
	}
	id3 := bead3.ID
	// Close the third one
	if err := c.Close(id3); err != nil {
		t.Skipf("Cannot close third bead: %v", err)
	}

	// Call ListWithLabel
	beads, err := c.ListWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error = %v", err)
	}

	// Build status map
	statusByID := make(map[string]string)
	for _, b := range beads {
		statusByID[b.ID] = b.Status
	}

	// Verify all three beads are present
	if _, found := statusByID[id1]; !found {
		t.Errorf("Expected to find open bead %s", id1)
	}
	if _, found := statusByID[id2]; !found {
		t.Errorf("Expected to find closed bead %s (missing --all flag)", id2)
	}
	if _, found := statusByID[id3]; !found {
		t.Errorf("Expected to find closed bead %s (missing --all flag)", id3)
	}

	// Verify the statuses are correct
	if statusByID[id1] != "open" {
		t.Errorf("Bead %s should have status=open, got %s", id1, statusByID[id1])
	}
	if statusByID[id2] != "closed" {
		t.Errorf("Bead %s should have status=closed, got %s", id2, statusByID[id2])
	}
	if statusByID[id3] != "closed" {
		t.Errorf("Bead %s should have status=closed, got %s", id3, statusByID[id3])
	}

	// Verify result count
	expectedCount := 3
	if len(beads) != expectedCount {
		t.Errorf("Expected %d beads (1 open + 2 closed), got %d. Without --all, only open beads are returned.", expectedCount, len(beads))
	}
}
