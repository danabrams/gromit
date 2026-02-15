package reviewpkg

import "testing"

// TestVariableDeclarationMergePattern documents that variable declarations
// followed immediately by assignment should be merged for clarity.
func TestVariableDeclarationMergePattern(t *testing.T) {
	// Good: merged declaration and assignment
	result := computeValue()
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}

	// This test verifies the pattern is correct
}

func computeValue() int {
	return 42
}
