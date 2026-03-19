package main

import (
	"strings"
	"testing"
)

// TestSpecListPreservesNonDoneOrder verifies non-done specs keep their original order.
func TestSpecListPreservesNonDoneOrder(t *testing.T) {
	specs := []string{"spec-1", "spec-2", "spec-3", "spec-4"}
	contents := map[string]string{
		"spec-1": "Regular spec",
		"spec-2": "DRAFT spec",
		"spec-3": "Another regular spec",
		"spec-4": "Yet another spec",
	}

	sorted := sortSpecsByDone(specs, contents)

	// All are non-done, order should be preserved
	for i, got := range sorted {
		if got != specs[i] {
			t.Errorf("position %d: got %q, expected %q", i, got, specs[i])
		}
	}
}

// TestSpecListStatusColumnContainsDone verifies status column includes done specs with dates.
func TestSpecListStatusColumnContainsDone(t *testing.T) {
	specs := []string{"spec-active", "spec-complete"}
	contents := map[string]string{
		"spec-active":   "Regular active spec",
		"spec-complete": "DONE 2026-03-10\nCompleted spec",
	}

	sorted := sortSpecsByDone(specs, contents)

	// Check that done spec comes after active
	if sorted[0] != "spec-active" || sorted[1] != "spec-complete" {
		t.Errorf("expected [spec-active, spec-complete], got %v", sorted)
	}

	// Verify formatting
	status := formatSpecStatusWithDate("spec-complete", nil, contents["spec-complete"])
	if !strings.Contains(status, "done (") {
		t.Errorf("expected status to contain 'done (', got %q", status)
	}
}
