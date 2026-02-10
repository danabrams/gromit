package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

// TestBeadClientInterface_AcceptanceReadyWithLabelMethod verifies that the BeadClient
// interface includes the ReadyWithLabel method and that all implementations satisfy it
func TestBeadClientInterface_AcceptanceReadyWithLabelMethod(t *testing.T) {
	// Verify that bead.Client satisfies BeadClient interface
	// This is a compile-time check - if ReadyWithLabel is missing, this won't compile
	var _ BeadClient = (*bead.Client)(nil)

	// Verify that mockBeadClient satisfies BeadClient interface
	var _ BeadClient = (*mockBeadClient)(nil)

	// Verify that MockBeadClientWithLabel satisfies BeadClient interface
	var _ BeadClient = (*MockBeadClientWithLabel)(nil)

	t.Log("✓ All implementations satisfy BeadClient interface with ReadyWithLabel method")
}

// TestBeadClientInterface_AcceptanceListWithLabelMethod verifies that the BeadClient
// interface includes the ListWithLabel method and that all implementations satisfy it
func TestBeadClientInterface_AcceptanceListWithLabelMethod(t *testing.T) {
	// Verify that bead.Client satisfies BeadClient interface
	var _ BeadClient = (*bead.Client)(nil)

	// Verify that mockBeadClient satisfies BeadClient interface
	var _ BeadClient = (*mockBeadClient)(nil)

	// Verify that MockBeadClientWithLabel satisfies BeadClient interface
	var _ BeadClient = (*MockBeadClientWithLabel)(nil)

	t.Log("✓ All implementations satisfy BeadClient interface with ListWithLabel method")
}

// TestMockBeadClient_AcceptanceReadyWithLabelReturnsConfiguredBead verifies that
// mockBeadClient.ReadyWithLabel returns the bead configured via ReadyWithLabelFn
func TestMockBeadClient_AcceptanceReadyWithLabelReturnsConfiguredBead(t *testing.T) {
	expectedBead := &bead.Bead{
		ID:       "test-bead-001",
		Title:    "Test bead for ReadyWithLabel",
		Priority: 1,
		Labels:   []string{"spec:auth", "complexity:high"},
	}

	mock := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			if label == "spec:auth" {
				return expectedBead, nil
			}
			return nil, nil
		},
	}

	// Test with matching label
	result, err := mock.ReadyWithLabel("spec:auth")
	if err != nil {
		t.Errorf("ReadyWithLabel() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("ReadyWithLabel() returned nil, expected bead")
	}
	if result.ID != expectedBead.ID {
		t.Errorf("ReadyWithLabel() returned bead with ID %q, expected %q", result.ID, expectedBead.ID)
	}
	if result.Title != expectedBead.Title {
		t.Errorf("ReadyWithLabel() returned bead with title %q, expected %q", result.Title, expectedBead.Title)
	}

	// Test with non-matching label
	result, err = mock.ReadyWithLabel("spec:payments")
	if err != nil {
		t.Errorf("ReadyWithLabel() with non-matching label returned error: %v", err)
	}
	if result != nil {
		t.Errorf("ReadyWithLabel() with non-matching label returned bead, expected nil")
	}
}

// TestMockBeadClient_AcceptanceReadyWithLabelReturnsNilWhenNotConfigured verifies that
// mockBeadClient.ReadyWithLabel returns nil when ReadyWithLabelFn is not set
func TestMockBeadClient_AcceptanceReadyWithLabelReturnsNilWhenNotConfigured(t *testing.T) {
	mock := &mockBeadClient{}

	result, err := mock.ReadyWithLabel("spec:test")
	if err != nil {
		t.Errorf("ReadyWithLabel() returned error when not configured: %v", err)
	}
	if result != nil {
		t.Errorf("ReadyWithLabel() returned bead when not configured, expected nil")
	}
}

// TestMockBeadClient_AcceptanceListWithLabelReturnsConfiguredBeads verifies that
// mockBeadClient.ListWithLabel returns the beads configured via ListWithLabelFn
func TestMockBeadClient_AcceptanceListWithLabelReturnsConfiguredBeads(t *testing.T) {
	expectedBeads := []*bead.Bead{
		{ID: "bead-1", Title: "First bead", Priority: 0, Labels: []string{"spec:auth"}},
		{ID: "bead-2", Title: "Second bead", Priority: 1, Labels: []string{"spec:auth"}},
		{ID: "bead-3", Title: "Third bead", Priority: 2, Labels: []string{"spec:auth"}},
	}

	mock := &mockBeadClient{
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			if label == "spec:auth" {
				return expectedBeads, nil
			}
			return []*bead.Bead{}, nil
		},
	}

	// Test with matching label
	result, err := mock.ListWithLabel("spec:auth")
	if err != nil {
		t.Fatalf("ListWithLabel() unexpected error: %v", err)
	}
	if len(result) != len(expectedBeads) {
		t.Errorf("ListWithLabel() returned %d beads, expected %d", len(result), len(expectedBeads))
	}
	for i, bead := range result {
		if bead.ID != expectedBeads[i].ID {
			t.Errorf("ListWithLabel() bead[%d] has ID %q, expected %q", i, bead.ID, expectedBeads[i].ID)
		}
		if bead.Title != expectedBeads[i].Title {
			t.Errorf("ListWithLabel() bead[%d] has title %q, expected %q", i, bead.Title, expectedBeads[i].Title)
		}
	}

	// Test with non-matching label
	result, err = mock.ListWithLabel("spec:payments")
	if err != nil {
		t.Errorf("ListWithLabel() with non-matching label returned error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("ListWithLabel() with non-matching label returned %d beads, expected 0", len(result))
	}
}

// TestMockBeadClient_AcceptanceListWithLabelReturnsEmptySliceWhenNotConfigured verifies
// that mockBeadClient.ListWithLabel returns an empty slice when ListWithLabelFn is not set
func TestMockBeadClient_AcceptanceListWithLabelReturnsEmptySliceWhenNotConfigured(t *testing.T) {
	mock := &mockBeadClient{}

	result, err := mock.ListWithLabel("spec:test")
	if err != nil {
		t.Errorf("ListWithLabel() returned error when not configured: %v", err)
	}
	if result == nil {
		t.Error("ListWithLabel() returned nil, expected empty slice")
	}
	if len(result) != 0 {
		t.Errorf("ListWithLabel() returned %d beads when not configured, expected 0", len(result))
	}
}

// TestBeadClientInterface_AcceptanceCanBeUsedInRunner verifies that the BeadClient
// interface with label methods can be used by the runner in typical scenarios
func TestBeadClientInterface_AcceptanceCanBeUsedInRunner(t *testing.T) {
	// Simulate a scenario where the runner needs to filter beads by label
	mock := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			if label == "spec:feature-x" {
				return &bead.Bead{
					ID:       "bead-feature-x",
					Title:    "Implement feature X",
					Priority: 1,
					Labels:   []string{"spec:feature-x"},
				}, nil
			}
			return nil, nil
		},
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			if label == "spec:feature-x" {
				return []*bead.Bead{
					{ID: "bead-1", Title: "Task 1", Priority: 1, Labels: []string{"spec:feature-x"}},
					{ID: "bead-2", Title: "Task 2", Priority: 2, Labels: []string{"spec:feature-x"}},
				}, nil
			}
			return []*bead.Bead{}, nil
		},
	}

	// Test that runner can call ReadyWithLabel through the interface
	var client BeadClient = mock
	bead, err := client.ReadyWithLabel("spec:feature-x")
	if err != nil {
		t.Fatalf("client.ReadyWithLabel() error: %v", err)
	}
	if bead == nil {
		t.Fatal("client.ReadyWithLabel() returned nil, expected bead")
	}
	if bead.ID != "bead-feature-x" {
		t.Errorf("client.ReadyWithLabel() returned bead %q, expected %q", bead.ID, "bead-feature-x")
	}

	// Test that runner can call ListWithLabel through the interface
	beads, err := client.ListWithLabel("spec:feature-x")
	if err != nil {
		t.Fatalf("client.ListWithLabel() error: %v", err)
	}
	if len(beads) != 2 {
		t.Errorf("client.ListWithLabel() returned %d beads, expected 2", len(beads))
	}
	if beads[0].ID != "bead-1" {
		t.Errorf("client.ListWithLabel() first bead has ID %q, expected %q", beads[0].ID, "bead-1")
	}
	if beads[1].ID != "bead-2" {
		t.Errorf("client.ListWithLabel() second bead has ID %q, expected %q", beads[1].ID, "bead-2")
	}
}

// TestMockBeadClient_AcceptanceReadyWithLabelSupportsErrorReturns verifies that
// mockBeadClient.ReadyWithLabel can return errors when configured to do so
func TestMockBeadClient_AcceptanceReadyWithLabelSupportsErrorReturns(t *testing.T) {
	expectedError := "bd CLI not available"

	mock := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			return nil, &mockError{msg: expectedError}
		},
	}

	_, err := mock.ReadyWithLabel("spec:test")
	if err == nil {
		t.Fatal("ReadyWithLabel() returned nil error, expected error")
	}
	if err.Error() != expectedError {
		t.Errorf("ReadyWithLabel() returned error %q, expected %q", err.Error(), expectedError)
	}
}

// TestMockBeadClient_AcceptanceListWithLabelSupportsErrorReturns verifies that
// mockBeadClient.ListWithLabel can return errors when configured to do so
func TestMockBeadClient_AcceptanceListWithLabelSupportsErrorReturns(t *testing.T) {
	expectedError := "bd list failed"

	mock := &mockBeadClient{
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			return nil, &mockError{msg: expectedError}
		},
	}

	_, err := mock.ListWithLabel("spec:test")
	if err == nil {
		t.Fatal("ListWithLabel() returned nil error, expected error")
	}
	if err.Error() != expectedError {
		t.Errorf("ListWithLabel() returned error %q, expected %q", err.Error(), expectedError)
	}
}

// TestMockBeadClientForStatus_AcceptanceHasLabelMethods verifies that the
// mockBeadClientForStatus used in Status() tests has the label methods
func TestMockBeadClientForStatus_AcceptanceHasLabelMethods(t *testing.T) {
	mock := &mockBeadClientForStatus{
		ready: &bead.Bead{ID: "status-bead", Title: "Status test"},
	}

	// Verify ReadyWithLabel exists and returns nil (default behavior)
	bead, err := mock.ReadyWithLabel("spec:test")
	if err != nil {
		t.Errorf("mockBeadClientForStatus.ReadyWithLabel() error: %v", err)
	}
	if bead != nil {
		t.Errorf("mockBeadClientForStatus.ReadyWithLabel() returned bead, expected nil")
	}

	// Verify ListWithLabel exists and returns empty slice (default behavior)
	beads, err := mock.ListWithLabel("spec:test")
	if err != nil {
		t.Errorf("mockBeadClientForStatus.ListWithLabel() error: %v", err)
	}
	if beads == nil {
		t.Error("mockBeadClientForStatus.ListWithLabel() returned nil, expected empty slice")
	}
	if len(beads) != 0 {
		t.Errorf("mockBeadClientForStatus.ListWithLabel() returned %d beads, expected 0", len(beads))
	}
}

// TestBeadClientInterface_AcceptanceLabelMethodsAreIndependent verifies that
// ReadyWithLabel and ListWithLabel are independent methods that can be called
// in any order and don't affect each other
func TestBeadClientInterface_AcceptanceLabelMethodsAreIndependent(t *testing.T) {
	readyCalled := false
	listCalled := false

	mock := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			readyCalled = true
			return &bead.Bead{ID: "ready-bead"}, nil
		},
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			listCalled = true
			return []*bead.Bead{{ID: "list-bead"}}, nil
		},
	}

	// Call ReadyWithLabel first
	_, err := mock.ReadyWithLabel("spec:test")
	if err != nil {
		t.Fatalf("ReadyWithLabel() error: %v", err)
	}
	if !readyCalled {
		t.Error("ReadyWithLabel() did not call ReadyWithLabelFn")
	}
	if listCalled {
		t.Error("ReadyWithLabel() incorrectly called ListWithLabelFn")
	}

	// Reset flags
	readyCalled = false
	listCalled = false

	// Call ListWithLabel second
	_, err = mock.ListWithLabel("spec:test")
	if err != nil {
		t.Fatalf("ListWithLabel() error: %v", err)
	}
	if readyCalled {
		t.Error("ListWithLabel() incorrectly called ReadyWithLabelFn")
	}
	if !listCalled {
		t.Error("ListWithLabel() did not call ListWithLabelFn")
	}
}

// TestBeadClientInterface_AcceptanceLabelMethodsHandleMultipleLabels verifies that
// the label methods can be called multiple times with different labels
func TestBeadClientInterface_AcceptanceLabelMethodsHandleMultipleLabels(t *testing.T) {
	calledLabels := []string{}

	mock := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			calledLabels = append(calledLabels, label)
			return &bead.Bead{ID: "bead-" + label}, nil
		},
	}

	// Call with different labels
	labels := []string{"spec:auth", "spec:payments", "spec:notifications"}
	for _, label := range labels {
		bead, err := mock.ReadyWithLabel(label)
		if err != nil {
			t.Errorf("ReadyWithLabel(%q) error: %v", label, err)
		}
		if bead == nil {
			t.Errorf("ReadyWithLabel(%q) returned nil", label)
		}
	}

	// Verify all labels were passed through
	if len(calledLabels) != len(labels) {
		t.Errorf("Expected %d label calls, got %d", len(labels), len(calledLabels))
	}
	for i, label := range labels {
		if i >= len(calledLabels) {
			break
		}
		if calledLabels[i] != label {
			t.Errorf("Call %d: expected label %q, got %q", i, label, calledLabels[i])
		}
	}
}

// mockError is a simple error type for testing error returns
type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}
