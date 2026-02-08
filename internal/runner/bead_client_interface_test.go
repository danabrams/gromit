package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

// TestBeadClientInterface_ListWithLabel tests that bead.Client implements ListWithLabel method
func TestBeadClientInterface_ListWithLabel(t *testing.T) {
	// Create a real client (implementation will fail without bd, but method should exist)
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("Failed to create bead client: %v", err)
	}

	// Verify the method exists and has correct signature by calling it
	// This will fail at runtime (bd not available), but it verifies the interface
	_, err = client.ListWithLabel("spec:test")

	// We expect an error because bd isn't running, but the method should exist
	// If the method doesn't exist, this won't compile
	if err == nil {
		// Unexpectedly succeeded - bd is running and returned data
		t.Log("ListWithLabel succeeded (bd is available)")
	} else {
		// Expected: bd command failed
		t.Logf("ListWithLabel failed as expected (bd not available): %v", err)
	}
}

// TestBeadClientInterfaceCompleteness tests that bead.Client satisfies BeadClient interface
func TestBeadClientInterfaceCompleteness(t *testing.T) {
	// This test verifies compile-time interface satisfaction
	// If bead.Client doesn't implement all BeadClient methods, this won't compile
	var _ BeadClient = (*bead.Client)(nil)

	// If we get here, the interface is satisfied
	t.Log("bead.Client implements BeadClient interface including ListWithLabel")
}

// TestBeadClientInterface_ListWithLabelReturnsSlice tests that ListWithLabel returns a slice of bead pointers
func TestBeadClientInterface_ListWithLabelReturnsSlice(t *testing.T) {
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("Failed to create bead client: %v", err)
	}

	// Call ListWithLabel and verify return type
	beads, err := client.ListWithLabel("spec:test")

	// Check that even with an error, beads is the correct type
	_ = beads // beads should be []*bead.Bead type

	// If beads was the wrong type, this wouldn't compile
	if err == nil {
		// bd is available and returned data
		if beads == nil {
			t.Error("ListWithLabel returned nil slice instead of empty slice")
		}
		t.Logf("ListWithLabel returned %d beads", len(beads))
	}
}
