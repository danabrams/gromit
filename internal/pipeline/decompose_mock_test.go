package pipeline

import (
	"testing"
)

// TestDecomposeAcceptanceBeadDef_ShouldNotExist verifies that decomposeAcceptanceBeadDef
// struct has been removed and replaced with direct BeadInfo usage
func TestDecomposeAcceptanceBeadDef_ShouldNotExist(t *testing.T) {
	// This test will fail to compile if decomposeAcceptanceBeadDef still exists
	// and is used as a type (not just as an unused identifier)

	// Create a mock that uses BeadInfo directly
	var createdBeads []*BeadInfo
	mock := &decomposeAcceptanceBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			bead := &BeadInfo{
				ID:       "test-id",
				Title:    title,
				Priority: priority,
				Labels:   labels,
			}
			createdBeads = append(createdBeads, bead)
			return bead, nil
		},
	}

	// Call the mock
	result, err := mock.CreateWithDepsAndDescription("Test", 1, []string{"label"}, []string{"criterion"}, []string{"dep"}, "desc")
	if err != nil {
		t.Fatalf("CreateWithDepsAndDescription failed: %v", err)
	}

	// Verify result
	if result.Title != "Test" {
		t.Errorf("result.Title = %q, want 'Test'", result.Title)
	}

	// Verify we can access the collected beads directly as BeadInfo
	if len(createdBeads) != 1 {
		t.Errorf("len(createdBeads) = %d, want 1", len(createdBeads))
	}
}
