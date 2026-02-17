package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

// setupTestBeadClient creates a bead client in a temporary directory for testing.
// This helper is extracted because multiple test functions need the same setup.
func setupTestBeadClient(t *testing.T) *bead.Client {
	tmpDir := t.TempDir()
	bdDir := filepath.Join(tmpDir, ".bd")
	if err := os.MkdirAll(bdDir, 0755); err != nil {
		t.Fatalf("failed to create .bd dir: %v", err)
	}

	// Initialize bd in the temp directory
	client := &bead.Client{
		Dir: tmpDir,
	}

	return client
}

// TestGetBeadCounts_ReturnsNonZeroClosedCount verifies that getBeadCounts correctly
// reports non-zero closed counts when closed beads exist with the queried label.
//
// Expected failure: Without --all flag in ListWithLabel, closed beads are not returned,
// so the closed count will always be 0.
func TestEpicBeadCountsReclassified_ClosedCountsIncludeAllStatuses(t *testing.T) {
	// Create a temporary bd directory for testing
	client := setupTestBeadClient(t)

	testSpec := "test-epic-counts"
	testLabel := "spec:" + testSpec

	// Create multiple beads with the test label
	openBead1, err := client.Create("Open task 1", 1, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create first open bead: %v", err)
	}

	closedBead1, err := client.Create("Closed task 1", 1, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create first closed bead: %v", err)
	}

	closedBead2, err := client.Create("Closed task 2", 1, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create second closed bead: %v", err)
	}

	openBead2, err := client.Create("Open task 2", 1, []string{testLabel}, []string{})
	if err != nil {
		t.Skipf("Cannot create second open bead: %v", err)
	}

	// Close two beads
	if err := client.Close(closedBead1.ID); err != nil {
		t.Skipf("Cannot close first bead: %v", err)
	}
	if err := client.Close(closedBead2.ID); err != nil {
		t.Skipf("Cannot close second bead: %v", err)
	}

	// Call getBeadCounts
	openCount, closedCount, err := getBeadCounts(client, testSpec)
	if err != nil {
		t.Fatalf("getBeadCounts() error = %v", err)
	}

	// Verify open count
	expectedOpen := 2
	if openCount != expectedOpen {
		t.Errorf("getBeadCounts() open count = %d, expected %d", openCount, expectedOpen)
	}

	// Verify closed count is non-zero
	expectedClosed := 2
	if closedCount == 0 {
		t.Errorf("getBeadCounts() closed count = 0, expected %d (without --all flag, ListWithLabel only returns open beads)", expectedClosed)
	}
	if closedCount != expectedClosed {
		t.Errorf("getBeadCounts() closed count = %d, expected %d", closedCount, expectedClosed)
	}

	// Log IDs for debugging
	t.Logf("Created beads: open=[%s, %s], closed=[%s, %s]", openBead1.ID, openBead2.ID, closedBead1.ID, closedBead2.ID)
	t.Logf("getBeadCounts returned: open=%d, closed=%d", openCount, closedCount)
}

// TestGetBeadCounts_HandlesOnlyClosedBeads verifies that getBeadCounts works correctly
// when all beads for a spec are closed.
//
// Expected failure: Without --all flag, ListWithLabel returns no beads when all are closed.
func TestEpicBeadCountsReclassified_AllClosedBeadsCounted(t *testing.T) {
	client := setupTestBeadClient(t)

	testSpec := "all-closed-spec"
	testLabel := "spec:" + testSpec

	// Create and close multiple beads
	const beadCount = 3
	for i := 0; i < beadCount; i++ {
		bead, err := client.Create("Task to be closed", 1, []string{testLabel}, []string{})
		if err != nil {
			t.Skipf("Cannot create bead %d: %v", i, err)
		}
		if err := client.Close(bead.ID); err != nil {
			t.Skipf("Cannot close bead %d: %v", i, err)
		}
	}

	// Call getBeadCounts
	openCount, closedCount, err := getBeadCounts(client, testSpec)
	if err != nil {
		t.Fatalf("getBeadCounts() error = %v", err)
	}

	// Verify counts
	if openCount != 0 {
		t.Errorf("getBeadCounts() open count = %d, expected 0", openCount)
	}
	if closedCount != beadCount {
		t.Errorf("getBeadCounts() closed count = %d, expected %d (without --all flag, no beads would be returned)", closedCount, beadCount)
	}
}

// TestGetBeadCounts_HandlesOnlyOpenBeads verifies that getBeadCounts works correctly
// when all beads for a spec are open (baseline case that works without --all flag).
func TestGetBeadCounts_HandlesOnlyOpenBeads(t *testing.T) {
	client := setupTestBeadClient(t)

	testSpec := "all-open-spec"
	testLabel := "spec:" + testSpec

	// Create multiple open beads
	const beadCount = 3
	for i := 0; i < beadCount; i++ {
		_, err := client.Create("Open task", 1, []string{testLabel}, []string{})
		if err != nil {
			t.Skipf("Cannot create bead %d: %v", i, err)
		}
	}

	// Call getBeadCounts
	openCount, closedCount, err := getBeadCounts(client, testSpec)
	if err != nil {
		t.Fatalf("getBeadCounts() error = %v", err)
	}

	// Verify counts
	if openCount != beadCount {
		t.Errorf("getBeadCounts() open count = %d, expected %d", openCount, beadCount)
	}
	if closedCount != 0 {
		t.Errorf("getBeadCounts() closed count = %d, expected 0", closedCount)
	}
}
