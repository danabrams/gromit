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

// TestRunner_GetNextBead_SingleLabelFilter verifies ReadyWithLabel is called with single filter
func TestRunner_GetNextBead_SingleLabelFilter(t *testing.T) {
	expectedBead := &bead.Bead{
		ID:       "test-002",
		Title:    "Auth task",
		Priority: 1,
		Labels:   []string{"spec:auth"},
		Type:     "task",
		Status:   "open",
	}

	var calledWithLabel string
	mock := &MockBeadClientWithLabel{
		ReadyWithLabelFunc: func(label string) (*bead.Bead, error) {
			calledWithLabel = label
			return expectedBead, nil
		},
		ReadyFunc: func() (*bead.Bead, error) {
			t.Error("Ready() should not be called when label filters are set")
			return nil, nil
		},
	}

	r := &Runner{
		beads:        mock,
		labelFilters: []string{"spec:auth"},
	}

	result, err := r.getNextBead()
	if err != nil {
		t.Fatalf("getNextBead() unexpected error: %v", err)
	}

	if calledWithLabel != "spec:auth" {
		t.Errorf("ReadyWithLabel() called with wrong label: got %s, want spec:auth", calledWithLabel)
	}

	if result == nil {
		t.Fatal("getNextBead() returned nil bead")
	}

	if result.ID != expectedBead.ID {
		t.Errorf("getNextBead() returned wrong bead: got %s, want %s", result.ID, expectedBead.ID)
	}
}

// TestRunner_GetNextBead_MultipleLabelFilters_FirstHasBead verifies all labels checked even if first has bead
func TestRunner_GetNextBead_MultipleLabelFilters_FirstHasBead(t *testing.T) {
	expectedBead := &bead.Bead{
		ID:       "test-003",
		Title:    "Auth task",
		Priority: 1,
		Labels:   []string{"spec:auth"},
		Type:     "task",
		Status:   "open",
	}

	callOrder := []string{}
	mock := &MockBeadClientWithLabel{
		ReadyWithLabelFunc: func(label string) (*bead.Bead, error) {
			callOrder = append(callOrder, label)
			if label == "spec:auth" {
				return expectedBead, nil
			}
			return nil, nil
		},
	}

	r := &Runner{
		beads:        mock,
		labelFilters: []string{"spec:auth", "spec:payments"},
	}

	result, err := r.getNextBead()
	if err != nil {
		t.Fatalf("getNextBead() unexpected error: %v", err)
	}

	// Should call both labels to collect all candidates
	if len(callOrder) != 2 {
		t.Errorf("Expected 2 calls to ReadyWithLabel, got %d calls: %v", len(callOrder), callOrder)
	}

	if callOrder[0] != "spec:auth" || callOrder[1] != "spec:payments" {
		t.Errorf("Expected calls [spec:auth, spec:payments], got %v", callOrder)
	}

	if result == nil {
		t.Fatal("getNextBead() returned nil bead")
	}

	if result.ID != expectedBead.ID {
		t.Errorf("getNextBead() returned wrong bead: got %s, want %s", result.ID, expectedBead.ID)
	}
}

// TestRunner_GetNextBead_MultipleLabelFilters_SecondHasBead verifies iteration continues to second label
func TestRunner_GetNextBead_MultipleLabelFilters_SecondHasBead(t *testing.T) {
	expectedBead := &bead.Bead{
		ID:       "test-004",
		Title:    "Payment task",
		Priority: 1,
		Labels:   []string{"spec:payments"},
		Type:     "task",
		Status:   "open",
	}

	callOrder := []string{}
	mock := &MockBeadClientWithLabel{
		ReadyWithLabelFunc: func(label string) (*bead.Bead, error) {
			callOrder = append(callOrder, label)
			if label == "spec:payments" {
				return expectedBead, nil
			}
			return nil, nil // spec:auth has no beads
		},
	}

	r := &Runner{
		beads:        mock,
		labelFilters: []string{"spec:auth", "spec:payments"},
	}

	result, err := r.getNextBead()
	if err != nil {
		t.Fatalf("getNextBead() unexpected error: %v", err)
	}

	if len(callOrder) != 2 {
		t.Errorf("Expected 2 calls to ReadyWithLabel, got %d calls: %v", len(callOrder), callOrder)
	}

	if callOrder[0] != "spec:auth" || callOrder[1] != "spec:payments" {
		t.Errorf("Expected calls in order [spec:auth, spec:payments], got %v", callOrder)
	}

	if result == nil {
		t.Fatal("getNextBead() returned nil bead")
	}

	if result.ID != expectedBead.ID {
		t.Errorf("getNextBead() returned wrong bead: got %s, want %s", result.ID, expectedBead.ID)
	}
}

// TestRunner_GetNextBead_MultipleLabelFilters_NoneHaveBeads verifies nil returned when no match
func TestRunner_GetNextBead_MultipleLabelFilters_NoneHaveBeads(t *testing.T) {
	callOrder := []string{}
	mock := &MockBeadClientWithLabel{
		ReadyWithLabelFunc: func(label string) (*bead.Bead, error) {
			callOrder = append(callOrder, label)
			return nil, nil // No beads for any label
		},
	}

	r := &Runner{
		beads:        mock,
		labelFilters: []string{"spec:auth", "spec:payments", "spec:reporting"},
	}

	result, err := r.getNextBead()
	if err != nil {
		t.Fatalf("getNextBead() unexpected error: %v", err)
	}

	if len(callOrder) != 3 {
		t.Errorf("Expected 3 calls to ReadyWithLabel, got %d calls: %v", len(callOrder), callOrder)
	}

	expected := []string{"spec:auth", "spec:payments", "spec:reporting"}
	for i, label := range expected {
		if i >= len(callOrder) || callOrder[i] != label {
			t.Errorf("Expected call %d to be %s, got %v", i, label, callOrder)
		}
	}

	if result != nil {
		t.Errorf("Expected nil bead when no labels have beads, got %+v", result)
	}
}

// TestRunner_GetNextBead_MultipleLabelFilters_PicksHighestPriority verifies highest priority wins
func TestRunner_GetNextBead_MultipleLabelFilters_PicksHighestPriority(t *testing.T) {
	authBead := &bead.Bead{
		ID:       "auth-001",
		Title:    "Auth task",
		Priority: 1, // P1
		Labels:   []string{"spec:auth"},
		Type:     "task",
		Status:   "open",
	}

	paymentBead := &bead.Bead{
		ID:       "pay-001",
		Title:    "Payment task",
		Priority: 0, // P0 - higher priority
		Labels:   []string{"spec:payments"},
		Type:     "task",
		Status:   "open",
	}

	callOrder := []string{}
	mock := &MockBeadClientWithLabel{
		ReadyWithLabelFunc: func(label string) (*bead.Bead, error) {
			callOrder = append(callOrder, label)
			if label == "spec:auth" {
				return authBead, nil
			}
			if label == "spec:payments" {
				return paymentBead, nil
			}
			return nil, nil
		},
	}

	r := &Runner{
		beads:        mock,
		labelFilters: []string{"spec:auth", "spec:payments"},
	}

	result, err := r.getNextBead()
	if err != nil {
		t.Fatalf("getNextBead() unexpected error: %v", err)
	}

	// Should call ReadyWithLabel for all labels to collect candidates
	if len(callOrder) != 2 {
		t.Errorf("Expected 2 calls to ReadyWithLabel, got %d calls: %v", len(callOrder), callOrder)
	}

	if result == nil {
		t.Fatal("getNextBead() returned nil bead")
	}

	// Should return the P0 payment bead (highest priority), not P1 auth bead
	if result.ID != paymentBead.ID {
		t.Errorf("getNextBead() should return highest priority bead (pay-001), got %s", result.ID)
	}
}
