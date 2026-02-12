//go:build acceptance

package runner

import (
	"errors"
	"sync"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

// TestAcceptance_MockBeadClientImplementsInterface verifies all mock implementations
// satisfy the BeadClient interface and label methods work correctly.
func TestAcceptance_MockBeadClientImplementsInterface(t *testing.T) {
	// Verify all mocks compile as BeadClient
	var _ BeadClient = (*mockBeadClient)(nil)
	var _ BeadClient = (*mockBeadClientForStatus)(nil)

	// Verify label methods are callable and error propagation works
	expectedErr := errors.New("test error")
	mock := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			if label == "" {
				return nil, errors.New("empty label")
			}
			if label == "error" {
				return nil, expectedErr
			}
			return &bead.Bead{ID: "test-001", Labels: []string{label}}, nil
		},
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			if label == "error" {
				return nil, expectedErr
			}
			return []*bead.Bead{{ID: "test-001", Labels: []string{label}}}, nil
		},
	}

	// Test ReadyWithLabel error handling
	_, err := mock.ReadyWithLabel("error")
	if err != expectedErr {
		t.Errorf("ReadyWithLabel() error = %v, want %v", err, expectedErr)
	}

	// Test ListWithLabel error handling
	_, err = mock.ListWithLabel("error")
	if err != expectedErr {
		t.Errorf("ListWithLabel() error = %v, want %v", err, expectedErr)
	}

	// Test successful calls
	b, err := mock.ReadyWithLabel("spec:test")
	if err != nil || b == nil || b.ID != "test-001" {
		t.Errorf("ReadyWithLabel() = %v, %v; want bead with ID test-001, nil error", b, err)
	}

	beads, err := mock.ListWithLabel("spec:test")
	if err != nil || len(beads) != 1 {
		t.Errorf("ListWithLabel() returned %d beads, %v; want 1 bead, nil error", len(beads), err)
	}
}

// TestAcceptance_MockBeadClientConcurrency verifies mocks are safe for concurrent use.
func TestAcceptance_MockBeadClientConcurrency(t *testing.T) {
	mock := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			return &bead.Bead{ID: "test-001", Labels: []string{label}}, nil
		},
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			return []*bead.Bead{{ID: "test-001"}}, nil
		},
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = mock.ReadyWithLabel("spec:test")
		}()
		go func() {
			defer wg.Done()
			_, _ = mock.ListWithLabel("spec:test")
		}()
	}
	wg.Wait()
}

// TestAcceptance_MocksHaveConsistentNilBehavior verifies all mocks handle nil
// function pointers consistently (no panic, return safe defaults).
func TestAcceptance_MocksHaveConsistentNilBehavior(t *testing.T) {
	mock := &mockBeadClient{} // All function pointers nil

	// Should not panic with nil function pointers
	b, err := mock.ReadyWithLabel("any")
	if b != nil || err != nil {
		t.Errorf("ReadyWithLabel() with nil Fn = %v, %v; want nil, nil", b, err)
	}

	// ListWithLabel returns empty slice, not nil - this is acceptable
	_, err = mock.ListWithLabel("any")
	if err != nil {
		t.Errorf("ListWithLabel() with nil Fn error = %v; want nil", err)
	}
}
