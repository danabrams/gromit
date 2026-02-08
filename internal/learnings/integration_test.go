package learnings

import (
	"path/filepath"
	"testing"
)

// TestLoadActualLearningsFile verifies that the project's LEARNINGS.md file can be parsed correctly
func TestLoadActualLearningsFile(t *testing.T) {
	// Get path to actual .gromit directory
	gromitDir := filepath.Join("..", "..", ".gromit")

	f, err := NewFile(gromitDir)
	if err != nil {
		t.Fatalf("Failed to create learnings file: %v", err)
	}

	if err := f.Load(); err != nil {
		t.Fatalf("Failed to load LEARNINGS.md: %v", err)
	}

	confirmed := f.GetConfirmed()
	if len(confirmed) < 6 {
		t.Errorf("Expected at least 6 confirmed learnings, got %d", len(confirmed))
	}

	// Verify the first few confirmed learnings have proper structure
	expectedTitles := []string{
		"Shell Safety",
		"Documentary Test Replacement",
		"Mock Implementation Patterns",
		"Status File Management",
		"Output Formatting",
		"LEARNINGS.md Format Validation",
	}

	for i, expected := range expectedTitles {
		if i >= len(confirmed) {
			break
		}
		l := confirmed[i]

		// Check that date is not zero value
		if l.Date.IsZero() {
			t.Errorf("Learning %d (%s) has zero date", i+1, expected)
		}

		// Check that BeadID matches expected title
		if l.BeadID != expected {
			t.Errorf("Learning %d: expected BeadID %q, got %q", i+1, expected, l.BeadID)
		}

		// Check that category is set
		if l.Category == "" {
			t.Errorf("Learning %d (%s) has empty category", i+1, expected)
		}

		// Check that RelatedTo is populated (these are consolidated learnings)
		if l.RelatedTo == "" {
			t.Errorf("Learning %d (%s) has empty RelatedTo field", i+1, expected)
		}

		t.Logf("Learning %d: %s | %s | %s | Related: %s",
			i+1, l.Date.Format("2006-01-02"), l.BeadID, l.Category, l.RelatedTo)
	}
}
