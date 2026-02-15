package runner

import "testing"

// TestTouchedPackagesNilSliceBehavior verifies that len() correctly handles
// nil slices, so explicit nil checks are unnecessary before len() calls.
func TestTouchedPackagesNilSliceBehavior(t *testing.T) {
	// This test documents that len(nil slice) == 0
	var nilSlice []string
	if len(nilSlice) != 0 {
		t.Errorf("expected len(nil slice) == 0, got %d", len(nilSlice))
	}

	// Empty slice also has length 0
	emptySlice := []string{}
	if len(emptySlice) != 0 {
		t.Errorf("expected len(empty slice) == 0, got %d", len(emptySlice))
	}
}
