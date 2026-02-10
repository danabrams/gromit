package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

// TestBeadClientInterface_AcceptanceReadyWithLabelExists verifies that the BeadClient
// interface includes the ReadyWithLabel method as required by the epic-scoped-execution specification
func TestBeadClientInterface_AcceptanceReadyWithLabelExists(t *testing.T) {
	// This test verifies compile-time satisfaction of the interface.
	// If ReadyWithLabel is missing from the interface, this will not compile.
	var client BeadClient = &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			return &bead.Bead{
				ID:              "test-1",
				Title:           "Test bead",
				Labels:          []string{label},
				ExpectedOutputs: []string{},
			}, nil
		},
	}

	// Call the method to ensure it's callable through the interface
	result, err := client.ReadyWithLabel("spec:test")
	if err != nil {
		t.Errorf("ReadyWithLabel() returned unexpected error: %v", err)
	}
	if result == nil {
		t.Error("ReadyWithLabel() returned nil, expected a bead")
	}
	if result != nil && !bead.HasLabel(result.Labels, "spec:test") {
		t.Errorf("ReadyWithLabel() returned bead without expected label, got: %v", result.Labels)
	}
}

// TestBeadClientInterface_AcceptanceListWithLabelExists verifies that the BeadClient
// interface includes the ListWithLabel method as required by the epic-scoped-execution specification
func TestBeadClientInterface_AcceptanceListWithLabelExists(t *testing.T) {
	// This test verifies compile-time satisfaction of the interface.
	// If ListWithLabel is missing from the interface, this will not compile.
	var client BeadClient = &mockBeadClient{
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			return []*bead.Bead{
				{
					ID:              "test-1",
					Title:           "Test bead 1",
					Labels:          []string{label},
					ExpectedOutputs: []string{},
				},
				{
					ID:              "test-2",
					Title:           "Test bead 2",
					Labels:          []string{label},
					ExpectedOutputs: []string{},
				},
			}, nil
		},
	}

	// Call the method to ensure it's callable through the interface
	results, err := client.ListWithLabel("spec:test")
	if err != nil {
		t.Errorf("ListWithLabel() returned unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("ListWithLabel() returned %d beads, expected 2", len(results))
	}
	for i, result := range results {
		if !bead.HasLabel(result.Labels, "spec:test") {
			t.Errorf("ListWithLabel() bead[%d] does not have expected label, got: %v", i, result.Labels)
		}
	}
}

// TestBeadClientInterface_AcceptanceRealClientImplementsReadyWithLabel verifies that
// the real bead.Client implementation satisfies the BeadClient interface with ReadyWithLabel
func TestBeadClientInterface_AcceptanceRealClientImplementsReadyWithLabel(t *testing.T) {
	// Compile-time check that bead.Client satisfies BeadClient
	// This is also checked in interfaces.go but we verify it here in acceptance tests
	var _ BeadClient = (*bead.Client)(nil)

	// Create a real client and verify the method exists
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("Failed to create bead client: %v", err)
	}

	// Attempt to call ReadyWithLabel - we expect this to fail with a bd command error
	// since we're not in a bd-initialized directory, but the method should exist
	_, err = client.ReadyWithLabel("spec:test")
	// We expect an error (bd not available or not initialized), but not a compile error
	// The presence of an error confirms the method exists and was called
	if err == nil {
		t.Log("ReadyWithLabel method exists and returned successfully (bd might be running)")
	} else {
		t.Logf("ReadyWithLabel method exists (expected bd error: %v)", err)
	}
}

// TestBeadClientInterface_AcceptanceRealClientImplementsListWithLabel verifies that
// the real bead.Client implementation satisfies the BeadClient interface with ListWithLabel
func TestBeadClientInterface_AcceptanceRealClientImplementsListWithLabel(t *testing.T) {
	// Create a real client and verify the method exists
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("Failed to create bead client: %v", err)
	}

	// Attempt to call ListWithLabel - we expect this to fail with a bd command error
	// since we're not in a bd-initialized directory, but the method should exist
	_, err = client.ListWithLabel("spec:test")
	// We expect an error (bd not available or not initialized), but not a compile error
	if err == nil {
		t.Log("ListWithLabel method exists and returned successfully (bd might be running)")
	} else {
		t.Logf("ListWithLabel method exists (expected bd error: %v)", err)
	}
}

// TestBeadClientInterface_AcceptanceMockBeadClientImplementsBothMethods verifies that
// the mockBeadClient test helper implements both new methods
func TestBeadClientInterface_AcceptanceMockBeadClientImplementsBothMethods(t *testing.T) {
	readyWithLabelCalled := false
	listWithLabelCalled := false

	mock := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			readyWithLabelCalled = true
			return &bead.Bead{
				ID:              "test-ready",
				Labels:          []string{label},
				ExpectedOutputs: []string{},
			}, nil
		},
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			listWithLabelCalled = true
			return []*bead.Bead{
				{ID: "test-list-1", Labels: []string{label}, ExpectedOutputs: []string{}},
				{ID: "test-list-2", Labels: []string{label}, ExpectedOutputs: []string{}},
			}, nil
		},
	}

	// Verify compile-time interface satisfaction
	var _ BeadClient = mock

	// Call both methods
	readyResult, err := mock.ReadyWithLabel("spec:test")
	if err != nil {
		t.Errorf("ReadyWithLabel() error: %v", err)
	}
	if !readyWithLabelCalled {
		t.Error("ReadyWithLabel() did not call the injected function")
	}
	if readyResult == nil || readyResult.ID != "test-ready" {
		t.Errorf("ReadyWithLabel() returned unexpected result: %+v", readyResult)
	}

	listResults, err := mock.ListWithLabel("spec:test")
	if err != nil {
		t.Errorf("ListWithLabel() error: %v", err)
	}
	if !listWithLabelCalled {
		t.Error("ListWithLabel() did not call the injected function")
	}
	if len(listResults) != 2 {
		t.Errorf("ListWithLabel() returned %d results, expected 2", len(listResults))
	}
}

// TestBeadClientInterface_AcceptanceMockBeadClientForStatusImplementsBothMethods verifies
// that the mockBeadClientForStatus used in runner status tests implements both new methods
func TestBeadClientInterface_AcceptanceMockBeadClientForStatusImplementsBothMethods(t *testing.T) {
	mock := &mockBeadClientForStatus{
		ready: &bead.Bead{
			ID:              "status-test",
			Labels:          []string{},
			ExpectedOutputs: []string{},
		},
	}

	// Verify compile-time interface satisfaction
	var _ BeadClient = mock

	// Call both new methods to ensure they exist and don't panic
	readyResult, err := mock.ReadyWithLabel("spec:test")
	if err != nil {
		t.Errorf("ReadyWithLabel() error: %v", err)
	}
	// mockBeadClientForStatus returns nil for ReadyWithLabel (minimal implementation)
	if readyResult != nil {
		t.Logf("ReadyWithLabel() returned: %+v", readyResult)
	}

	listResults, err := mock.ListWithLabel("spec:test")
	if err != nil {
		t.Errorf("ListWithLabel() error: %v", err)
	}
	// mockBeadClientForStatus returns empty slice for ListWithLabel (minimal implementation)
	if listResults == nil {
		t.Error("ListWithLabel() returned nil, expected empty slice")
	}
	if len(listResults) != 0 {
		t.Logf("ListWithLabel() returned %d results", len(listResults))
	}
}

// TestBeadClientInterface_AcceptanceReadyWithLabelSignature verifies the exact signature
// of ReadyWithLabel matches the specification requirements
func TestBeadClientInterface_AcceptanceReadyWithLabelSignature(t *testing.T) {
	// This test verifies the method signature is exactly:
	//   ReadyWithLabel(label string) (*bead.Bead, error)
	//
	// If the signature changes, this test will fail to compile

	mock := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			// Verify the label parameter is a string
			if len(label) == 0 {
				t.Error("label parameter is empty")
			}
			// Return the correct types
			return &bead.Bead{ID: "sig-test", Labels: []string{}, ExpectedOutputs: []string{}}, nil
		},
	}

	var client BeadClient = mock

	// Call with string parameter, expect *bead.Bead and error return
	result, err := client.ReadyWithLabel("spec:signature-test")

	// Verify return types
	if err != nil {
		t.Logf("Got error (acceptable): %v", err)
	}
	if result != nil {
		if result.ID != "sig-test" {
			t.Errorf("Expected ID 'sig-test', got %q", result.ID)
		}
	}
}

// TestBeadClientInterface_AcceptanceListWithLabelSignature verifies the exact signature
// of ListWithLabel matches the specification requirements
func TestBeadClientInterface_AcceptanceListWithLabelSignature(t *testing.T) {
	// This test verifies the method signature is exactly:
	//   ListWithLabel(label string) ([]*bead.Bead, error)
	//
	// If the signature changes, this test will fail to compile

	mock := &mockBeadClient{
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			// Verify the label parameter is a string
			if len(label) == 0 {
				t.Error("label parameter is empty")
			}
			// Return the correct types
			return []*bead.Bead{
				{ID: "sig-test-1", Labels: []string{}, ExpectedOutputs: []string{}},
				{ID: "sig-test-2", Labels: []string{}, ExpectedOutputs: []string{}},
			}, nil
		},
	}

	var client BeadClient = mock

	// Call with string parameter, expect []*bead.Bead and error return
	results, err := client.ListWithLabel("spec:signature-test")

	// Verify return types
	if err != nil {
		t.Logf("Got error (acceptable): %v", err)
	}
	if results == nil {
		t.Error("Expected non-nil slice, got nil")
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

// TestBeadClientInterface_AcceptanceAllMocksUseFnFieldPattern verifies that all
// mock implementations use the consistent FnField pattern for optional behavior injection
func TestBeadClientInterface_AcceptanceAllMocksUseFnFieldPattern(t *testing.T) {
	// Verify mockBeadClient follows the pattern
	mock1 := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			return &bead.Bead{ID: "test-1", Labels: []string{}, ExpectedOutputs: []string{}}, nil
		},
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			return []*bead.Bead{{ID: "test-2", Labels: []string{}, ExpectedOutputs: []string{}}}, nil
		},
	}

	// Test nil-safe defaults (when Fn fields are not set)
	mock2 := &mockBeadClient{}

	// Both should implement the interface
	var _ BeadClient = mock1
	var _ BeadClient = mock2

	// Call with nil Fn fields - should return nil-safe defaults
	result1, err := mock2.ReadyWithLabel("spec:test")
	if err != nil {
		t.Errorf("Expected nil error for nil Fn field, got: %v", err)
	}
	if result1 != nil {
		t.Logf("Default behavior returned: %+v", result1)
	}

	results2, err := mock2.ListWithLabel("spec:test")
	if err != nil {
		t.Errorf("Expected nil error for nil Fn field, got: %v", err)
	}
	if results2 == nil {
		t.Error("Expected empty slice for nil Fn field, got nil")
	}
}

// TestBeadClientInterface_AcceptanceCompatibilityWithExistingCode verifies that
// adding the new methods doesn't break existing code patterns
func TestBeadClientInterface_AcceptanceCompatibilityWithExistingCode(t *testing.T) {
	// Verify that existing code using Ready() still works alongside ReadyWithLabel()
	mock := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return &bead.Bead{
				ID:              "existing-ready",
				Labels:          []string{},
				ExpectedOutputs: []string{},
			}, nil
		},
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			return &bead.Bead{
				ID:              "new-ready-with-label",
				Labels:          []string{label},
				ExpectedOutputs: []string{},
			}, nil
		},
	}

	var client BeadClient = mock

	// Existing code path (Ready without label)
	existingResult, err := client.Ready()
	if err != nil {
		t.Errorf("Ready() error: %v", err)
	}
	if existingResult == nil || existingResult.ID != "existing-ready" {
		t.Error("Ready() method is broken after adding ReadyWithLabel()")
	}

	// New code path (Ready with label filter)
	newResult, err := client.ReadyWithLabel("spec:test")
	if err != nil {
		t.Errorf("ReadyWithLabel() error: %v", err)
	}
	if newResult == nil || newResult.ID != "new-ready-with-label" {
		t.Error("ReadyWithLabel() is not working correctly")
	}

	// Verify both methods can coexist and return different results
	if existingResult != nil && newResult != nil && existingResult.ID == newResult.ID {
		t.Error("Ready() and ReadyWithLabel() should be independent methods")
	}
}
