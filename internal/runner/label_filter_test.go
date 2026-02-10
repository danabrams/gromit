package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

// TestRunner_SetLabelFilters verifies that SetLabelFilters stores label filters correctly
func TestRunner_SetLabelFilters(t *testing.T) {
	r := &Runner{}

	labels := []string{"spec:auth", "spec:payments"}
	r.SetLabelFilters(labels)

	if len(r.labelFilters) != 2 {
		t.Errorf("Expected 2 label filters, got %d", len(r.labelFilters))
	}
	if r.labelFilters[0] != "spec:auth" {
		t.Errorf("Expected first label to be 'spec:auth', got %s", r.labelFilters[0])
	}
	if r.labelFilters[1] != "spec:payments" {
		t.Errorf("Expected second label to be 'spec:payments', got %s", r.labelFilters[1])
	}
}

// TestRunner_GetNextBead_NoFilters verifies that with no filters, Ready() is called
func TestRunner_GetNextBead_NoFilters(t *testing.T) {
	expectedBead := &bead.Bead{
		ID:       "test-001",
		Title:    "Test task",
		Priority: 1,
		Labels:   []string{},
		Type:     "task",
		Status:   "open",
	}

	readyCalled := false
	mock := &MockBeadClientWithLabel{
		ReadyFunc: func() (*bead.Bead, error) {
			readyCalled = true
			return expectedBead, nil
		},
	}

	r := &Runner{
		beads:        mock,
		labelFilters: []string{}, // No filters
	}

	result, err := r.getNextBead()
	if err != nil {
		t.Fatalf("getNextBead() unexpected error: %v", err)
	}

	if !readyCalled {
		t.Error("Expected Ready() to be called when no label filters are set")
	}

	if result == nil {
		t.Fatal("getNextBead() returned nil bead")
	}

	if result.ID != expectedBead.ID {
		t.Errorf("getNextBead() returned wrong bead: got %s, want %s", result.ID, expectedBead.ID)
	}
}
