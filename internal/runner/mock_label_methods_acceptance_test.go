//go:build acceptance

package runner

import (
	"errors"
	"sync"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

// TestAcceptance_MockBeadClientReadyWithLabelErrorHandling verifies that
// mockBeadClient's ReadyWithLabel correctly propagates errors from the callback.
func TestAcceptance_MockBeadClientReadyWithLabelErrorHandling(t *testing.T) {
	expectedErr := errors.New("simulated bd ready error")
	mock := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			return nil, expectedErr
		},
	}

	result, err := mock.ReadyWithLabel("spec:test")

	if err != expectedErr {
		t.Errorf("ReadyWithLabel() error = %v, want %v", err, expectedErr)
	}
	if result != nil {
		t.Errorf("ReadyWithLabel() returned bead %v, want nil when error occurs", result)
	}
}

// TestAcceptance_MockBeadClientListWithLabelErrorHandling verifies that
// mockBeadClient's ListWithLabel correctly propagates errors from the callback.
func TestAcceptance_MockBeadClientListWithLabelErrorHandling(t *testing.T) {
	expectedErr := errors.New("simulated bd list error")
	mock := &mockBeadClient{
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			return nil, expectedErr
		},
	}

	results, err := mock.ListWithLabel("spec:test")

	if err != expectedErr {
		t.Errorf("ListWithLabel() error = %v, want %v", err, expectedErr)
	}
	if results != nil {
		t.Errorf("ListWithLabel() returned %v, want nil when error occurs", results)
	}
}

// TestAcceptance_MockBeadClientLabelMethodsConcurrentUse verifies that
// mockBeadClient's label methods can be called concurrently without panics.
func TestAcceptance_MockBeadClientLabelMethodsConcurrentUse(t *testing.T) {
	mock := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			return &bead.Bead{ID: "concurrent-test", Title: "Concurrent Bead"}, nil
		},
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			return []*bead.Bead{
				{ID: "bead-1", Title: "First"},
				{ID: "bead-2", Title: "Second"},
			}, nil
		},
	}

	var wg sync.WaitGroup
	iterations := 10

	// Test concurrent ReadyWithLabel calls
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			_, err := mock.ReadyWithLabel("spec:concurrent")
			if err != nil {
				t.Errorf("Concurrent ReadyWithLabel() call %d failed: %v", iteration, err)
			}
		}(i)
	}

	// Test concurrent ListWithLabel calls
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			_, err := mock.ListWithLabel("spec:concurrent")
			if err != nil {
				t.Errorf("Concurrent ListWithLabel() call %d failed: %v", iteration, err)
			}
		}(i)
	}

	wg.Wait()
}

// TestAcceptance_MockBeadClientLabelMethodsWithAllInterfaceMethods verifies
// that label methods work correctly when used alongside other BeadClient methods.
func TestAcceptance_MockBeadClientLabelMethodsWithAllInterfaceMethods(t *testing.T) {
	var closedBeads []string
	var shownBeads []string

	mock := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			if label == "spec:feature-x" {
				return &bead.Bead{ID: "feature-x-1", Title: "Feature X Task"}, nil
			}
			return nil, nil
		},
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			if label == "spec:feature-x" {
				return []*bead.Bead{
					{ID: "feature-x-1", Title: "Task 1"},
					{ID: "feature-x-2", Title: "Task 2"},
				}, nil
			}
			return []*bead.Bead{}, nil
		},
		ShowFn: func(id string) (*bead.Bead, error) {
			shownBeads = append(shownBeads, id)
			return &bead.Bead{ID: id, Title: "Shown Bead"}, nil
		},
		CloseFn: func(id string) error {
			closedBeads = append(closedBeads, id)
			return nil
		},
	}

	// Use ReadyWithLabel to get a bead
	readyBead, err := mock.ReadyWithLabel("spec:feature-x")
	if err != nil {
		t.Fatalf("ReadyWithLabel() failed: %v", err)
	}
	if readyBead == nil {
		t.Fatal("ReadyWithLabel() returned nil bead")
	}

	// Use Show to get details
	shownBead, err := mock.Show(readyBead.ID)
	if err != nil {
		t.Fatalf("Show() failed: %v", err)
	}
	if shownBead == nil {
		t.Fatal("Show() returned nil bead")
	}

	// Use Close to mark it complete
	err = mock.Close(readyBead.ID)
	if err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// Use ListWithLabel to get remaining beads
	remainingBeads, err := mock.ListWithLabel("spec:feature-x")
	if err != nil {
		t.Fatalf("ListWithLabel() failed: %v", err)
	}

	// Verify all operations recorded correctly
	if len(shownBeads) != 1 || shownBeads[0] != "feature-x-1" {
		t.Errorf("Show() was not called correctly, got %v", shownBeads)
	}
	if len(closedBeads) != 1 || closedBeads[0] != "feature-x-1" {
		t.Errorf("Close() was not called correctly, got %v", closedBeads)
	}
	if len(remainingBeads) != 2 {
		t.Errorf("ListWithLabel() returned %d beads, want 2", len(remainingBeads))
	}
}

// TestAcceptance_MockBeadClientForStatusLabelMethodsAreNoops verifies that
// mockBeadClientForStatus label methods return safe defaults and don't affect
// the primary Ready() behavior.
func TestAcceptance_MockBeadClientForStatusLabelMethodsAreNoops(t *testing.T) {
	expectedBead := &bead.Bead{ID: "status-test", Title: "Status Bead"}
	mock := &mockBeadClientForStatus{
		ready: expectedBead,
		err:   nil,
	}

	// Verify primary Ready() still works
	readyResult, err := mock.Ready()
	if err != nil {
		t.Errorf("Ready() returned error: %v", err)
	}
	if readyResult != expectedBead {
		t.Errorf("Ready() returned %v, want %v", readyResult, expectedBead)
	}

	// Verify label methods don't interfere
	labelReady, err := mock.ReadyWithLabel("any-label")
	if err != nil {
		t.Errorf("ReadyWithLabel() returned error: %v", err)
	}
	if labelReady != nil {
		t.Errorf("ReadyWithLabel() returned %v, want nil", labelReady)
	}

	labelList, err := mock.ListWithLabel("any-label")
	if err != nil {
		t.Errorf("ListWithLabel() returned error: %v", err)
	}
	if len(labelList) != 0 {
		t.Errorf("ListWithLabel() returned %d beads, want 0", len(labelList))
	}

	// Verify primary Ready() still works after label method calls
	readyResult2, err := mock.Ready()
	if err != nil {
		t.Errorf("Ready() after label calls returned error: %v", err)
	}
	if readyResult2 != expectedBead {
		t.Errorf("Ready() after label calls returned %v, want %v", readyResult2, expectedBead)
	}
}

// TestAcceptance_MockBeadClientLabelMethodsWithEmptyLabel verifies that
// label methods handle empty label strings correctly.
func TestAcceptance_MockBeadClientLabelMethodsWithEmptyLabel(t *testing.T) {
	mock := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			if label == "" {
				return &bead.Bead{ID: "empty-label-bead"}, nil
			}
			return nil, nil
		},
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			if label == "" {
				return []*bead.Bead{{ID: "bead-1"}}, nil
			}
			return []*bead.Bead{}, nil
		},
	}

	// Test ReadyWithLabel with empty string
	readyResult, err := mock.ReadyWithLabel("")
	if err != nil {
		t.Errorf("ReadyWithLabel(\"\") returned error: %v", err)
	}
	if readyResult == nil {
		t.Error("ReadyWithLabel(\"\") returned nil, expected bead")
	}

	// Test ListWithLabel with empty string
	listResult, err := mock.ListWithLabel("")
	if err != nil {
		t.Errorf("ListWithLabel(\"\") returned error: %v", err)
	}
	if len(listResult) != 1 {
		t.Errorf("ListWithLabel(\"\") returned %d beads, want 1", len(listResult))
	}
}

// TestAcceptance_AllMockBeadClientsHaveConsistentNilBehavior verifies that
// all mock implementations return consistent nil/empty values when callbacks are nil.
func TestAcceptance_AllMockBeadClientsHaveConsistentNilBehavior(t *testing.T) {
	tests := []struct {
		name   string
		client BeadClient
	}{
		{
			name:   "mockBeadClient",
			client: &mockBeadClient{},
		},
		{
			name:   "mockBeadClientForStatus",
			client: &mockBeadClientForStatus{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ReadyWithLabel with nil callback should not panic
			readyResult, readyErr := tt.client.ReadyWithLabel("test-label")
			if readyErr != nil {
				t.Errorf("%s ReadyWithLabel() with nil callback returned error: %v", tt.name, readyErr)
			}
			if readyResult != nil && tt.name != "mockBeadClient" {
				// mockBeadClient returns nil, mockBeadClientForStatus may have different behavior
				t.Logf("%s ReadyWithLabel() returned non-nil result: %v (may be expected)", tt.name, readyResult)
			}

			// ListWithLabel with nil callback should return empty slice
			listResult, listErr := tt.client.ListWithLabel("test-label")
			if listErr != nil {
				t.Errorf("%s ListWithLabel() with nil callback returned error: %v", tt.name, listErr)
			}
			if listResult == nil {
				t.Errorf("%s ListWithLabel() returned nil, want empty slice", tt.name)
			}
			if len(listResult) != 0 {
				t.Errorf("%s ListWithLabel() returned %d beads, want 0", tt.name, len(listResult))
			}
		})
	}
}
