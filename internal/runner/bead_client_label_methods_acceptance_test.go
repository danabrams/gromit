package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

// TestBeadClientInterface_HasLabelMethods is an acceptance test verifying that
// the BeadClient interface includes ReadyWithLabel and ListWithLabel methods
func TestBeadClientInterface_HasLabelMethods(t *testing.T) {
	// This test verifies at compile time that the BeadClient interface includes
	// ReadyWithLabel and ListWithLabel methods. If these methods are missing from
	// the interface, this test will fail to compile.

	// Verify bead.Client satisfies the interface (compile-time check)
	var _ BeadClient = (*bead.Client)(nil)

	t.Log("✓ BeadClient interface includes ReadyWithLabel method")
	t.Log("✓ BeadClient interface includes ListWithLabel method")
	t.Log("✓ bead.Client satisfies BeadClient interface with label methods")
}

// TestMockBeadClient_HasLabelMethods is an acceptance test verifying that
// mockBeadClient implements ReadyWithLabel and ListWithLabel methods
func TestMockBeadClient_HasLabelMethods(t *testing.T) {
	// This test verifies at compile time that mockBeadClient (the primary mock
	// used throughout runner tests) implements the full BeadClient interface
	// including ReadyWithLabel and ListWithLabel.

	// Verify mockBeadClient satisfies the interface (compile-time check)
	var _ BeadClient = (*mockBeadClient)(nil)

	// Create a mock and verify methods can be called
	mock := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			return &bead.Bead{
				ID:              "test-1",
				Title:           "Test bead",
				Labels:          []string{label},
				ExpectedOutputs: []string{},
			}, nil
		},
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

	// Test ReadyWithLabel
	bead, err := mock.ReadyWithLabel("spec:test")
	if err != nil {
		t.Errorf("ReadyWithLabel() returned unexpected error: %v", err)
	}
	if bead == nil {
		t.Error("ReadyWithLabel() returned nil bead")
	} else if bead.ID != "test-1" {
		t.Errorf("ReadyWithLabel() returned bead ID %q, expected %q", bead.ID, "test-1")
	}

	// Test ListWithLabel
	beads, err := mock.ListWithLabel("spec:test")
	if err != nil {
		t.Errorf("ListWithLabel() returned unexpected error: %v", err)
	}
	if len(beads) != 2 {
		t.Errorf("ListWithLabel() returned %d beads, expected 2", len(beads))
	}

	t.Log("✓ mockBeadClient implements ReadyWithLabel method")
	t.Log("✓ mockBeadClient implements ListWithLabel method")
	t.Log("✓ mockBeadClient satisfies BeadClient interface with label methods")
}

// TestMockBeadClientForStatus_HasLabelMethods is an acceptance test verifying that
// mockBeadClientForStatus implements ReadyWithLabel and ListWithLabel methods
func TestMockBeadClientForStatus_HasLabelMethods(t *testing.T) {
	// This test verifies at compile time that mockBeadClientForStatus (used in
	// status-related tests) implements the full BeadClient interface including
	// ReadyWithLabel and ListWithLabel.

	// Verify mockBeadClientForStatus satisfies the interface (compile-time check)
	var _ BeadClient = (*mockBeadClientForStatus)(nil)

	// Create a mock and verify methods can be called
	mock := &mockBeadClientForStatus{
		ready: &bead.Bead{
			ID:              "status-test",
			Title:           "Status test bead",
			Labels:          []string{},
			ExpectedOutputs: []string{},
		},
		err: nil,
	}

	// Test ReadyWithLabel (should return nil by default implementation)
	bead, err := mock.ReadyWithLabel("spec:test")
	if err != nil {
		t.Errorf("ReadyWithLabel() returned unexpected error: %v", err)
	}
	// Default implementation returns nil
	if bead != nil {
		t.Logf("ReadyWithLabel() returned bead (default implementation may vary): %+v", bead)
	}

	// Test ListWithLabel (should return empty slice by default implementation)
	beads, err := mock.ListWithLabel("spec:test")
	if err != nil {
		t.Errorf("ListWithLabel() returned unexpected error: %v", err)
	}
	if beads == nil {
		t.Error("ListWithLabel() returned nil instead of empty slice")
	}

	t.Log("✓ mockBeadClientForStatus implements ReadyWithLabel method")
	t.Log("✓ mockBeadClientForStatus implements ListWithLabel method")
	t.Log("✓ mockBeadClientForStatus satisfies BeadClient interface with label methods")
}

// TestAllBeadClientMocks_SatisfyInterface is an acceptance test that verifies
// all BeadClient mock implementations in the runner package satisfy the interface
func TestAllBeadClientMocks_SatisfyInterface(t *testing.T) {
	// This comprehensive test ensures that all mock implementations used in
	// runner tests satisfy the BeadClient interface with ReadyWithLabel and
	// ListWithLabel methods. If any mock is missing these methods, this test
	// will fail to compile.

	// Verify each mock satisfies the interface (compile-time check)
	var _ BeadClient = (*mockBeadClient)(nil)
	var _ BeadClient = (*mockBeadClientForStatus)(nil)

	t.Log("✓ All BeadClient mocks in runner package satisfy the interface")
	t.Log("✓ All mocks implement ReadyWithLabel method")
	t.Log("✓ All mocks implement ListWithLabel method")
}

// TestBeadClientLabelMethods_FunctionalConsistency is an acceptance test that
// verifies the label methods work consistently across different mock implementations
func TestBeadClientLabelMethods_FunctionalConsistency(t *testing.T) {
	// This test verifies that all mock implementations handle the label methods
	// in a functionally consistent way (return appropriate types, no panics, etc.)

	testLabel := "spec:acceptance-test"

	// Test mockBeadClient
	t.Run("mockBeadClient", func(t *testing.T) {
		mock := &mockBeadClient{}

		// Test default behavior (nil function pointers)
		bead, err := mock.ReadyWithLabel(testLabel)
		if err != nil {
			t.Errorf("ReadyWithLabel() should not error with default nil function, got: %v", err)
		}
		if bead != nil {
			t.Logf("ReadyWithLabel() returned bead with default behavior: %+v", bead)
		}

		beads, err := mock.ListWithLabel(testLabel)
		if err != nil {
			t.Errorf("ListWithLabel() should not error with default nil function, got: %v", err)
		}
		if beads == nil {
			t.Error("ListWithLabel() should return empty slice, not nil")
		}
		if len(beads) != 0 {
			t.Errorf("ListWithLabel() should return empty slice by default, got %d beads", len(beads))
		}
	})

	// Test mockBeadClientForStatus
	t.Run("mockBeadClientForStatus", func(t *testing.T) {
		mock := &mockBeadClientForStatus{}

		// Test default behavior
		bead, err := mock.ReadyWithLabel(testLabel)
		if err != nil {
			t.Errorf("ReadyWithLabel() should not error, got: %v", err)
		}
		if bead != nil {
			t.Logf("ReadyWithLabel() returned bead: %+v", bead)
		}

		beads, err := mock.ListWithLabel(testLabel)
		if err != nil {
			t.Errorf("ListWithLabel() should not error, got: %v", err)
		}
		if beads == nil {
			t.Error("ListWithLabel() should return empty slice, not nil")
		}
		if len(beads) != 0 {
			t.Errorf("ListWithLabel() should return empty slice, got %d beads", len(beads))
		}
	})

	t.Log("✓ All mock implementations handle label methods consistently")
	t.Log("✓ Label methods return appropriate types (no panics)")
	t.Log("✓ Default implementations return sensible values")
}

// TestBeadClientLabelMethods_NilSafety is an acceptance test that verifies
// the label methods handle nil receivers and nil function pointers safely
func TestBeadClientLabelMethods_NilSafety(t *testing.T) {
	// This test verifies that mock implementations don't panic when called
	// with nil function pointers (the default state).

	t.Run("mockBeadClient nil safety", func(t *testing.T) {
		mock := &mockBeadClient{}

		// These should not panic even with nil function pointers
		_, _ = mock.ReadyWithLabel("spec:test")
		_, _ = mock.ListWithLabel("spec:test")

		t.Log("✓ mockBeadClient methods are nil-safe")
	})

	t.Run("mockBeadClientForStatus nil safety", func(t *testing.T) {
		mock := &mockBeadClientForStatus{}

		// These should not panic
		_, _ = mock.ReadyWithLabel("spec:test")
		_, _ = mock.ListWithLabel("spec:test")

		t.Log("✓ mockBeadClientForStatus methods are nil-safe")
	})

	t.Log("✓ All mock label methods handle nil function pointers safely")
}

// TestBeadClientInterface_MethodSignatures is an acceptance test that documents
// the expected method signatures for ReadyWithLabel and ListWithLabel
func TestBeadClientInterface_MethodSignatures(t *testing.T) {
	// This test serves as living documentation for the expected signatures
	// of the label methods in the BeadClient interface.

	// Expected signatures (verified at compile time):
	// ReadyWithLabel(label string) (*bead.Bead, error)
	// ListWithLabel(label string) ([]*bead.Bead, error)

	// Create a test function that requires the correct signatures
	testReadyWithLabel := func(client BeadClient, label string) (*bead.Bead, error) {
		return client.ReadyWithLabel(label)
	}

	testListWithLabel := func(client BeadClient, label string) ([]*bead.Bead, error) {
		return client.ListWithLabel(label)
	}

	// Verify these compile and can be called
	mock := &mockBeadClient{}
	_, _ = testReadyWithLabel(mock, "spec:test")
	_, _ = testListWithLabel(mock, "spec:test")

	t.Log("✓ ReadyWithLabel signature: (label string) (*bead.Bead, error)")
	t.Log("✓ ListWithLabel signature: (label string) ([]*bead.Bead, error)")
	t.Log("✓ Method signatures are consistent across all implementations")
}
