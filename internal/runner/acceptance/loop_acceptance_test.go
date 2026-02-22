//go:build acceptance

package acceptance_test

import (
	"context"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
)

// TestOrchestrator_UsesLabelFiltersInLoop tests that OrchestratorTestHelper calls
// ReadyWithLabel for each label and not Ready when label filters are set.
func TestOrchestrator_UsesLabelFiltersInLoop(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	var queriedLabels []string
	callCount := 0
	mock := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			t.Error("Ready() should not be called when label filters are set")
			return nil, nil
		},
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			queriedLabels = append(queriedLabels, label)
			callCount++
			if callCount == 1 && label == "spec:auth" {
				return &bead.Bead{
					ID:              "auth-1",
					Title:           "Auth task",
					Priority:        1,
					Labels:          []string{"spec:auth"},
					ExpectedOutputs: []string{},
				}, nil
			}
			return nil, nil
		},
	}

	h := NewOrchestratorTestHelperWithDeps(t, cfg, io.Discard, mock, newMockRouter())
	h.SetLabelFilters([]string{"spec:auth", "spec:payments"})

	err := h.Run(context.Background(), 0, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(queriedLabels) < 2 {
		t.Fatalf("Expected at least 2 ReadyWithLabel calls, got %d: %v", len(queriedLabels), queriedLabels)
	}
	if !slices.Contains(queriedLabels, "spec:auth") {
		t.Error("Expected 'spec:auth' to be queried")
	}
	if !slices.Contains(queriedLabels, "spec:payments") {
		t.Error("Expected 'spec:payments' to be queried")
	}
}

// TestOrchestratorScopedRun_FullLoopWithLabelFilters tests that OrchestratorTestHelper
// processes only beads matching the label filters and never calls Ready().
func TestOrchestratorScopedRun_FullLoopWithLabelFilters(t *testing.T) {
	allBeads := []*bead.Bead{
		{ID: "auth-1", Title: "Auth task 1", Priority: 1, Labels: []string{"spec:auth"}, ExpectedOutputs: []string{}},
		{ID: "auth-2", Title: "Auth task 2", Priority: 0, Labels: []string{"spec:auth"}, ExpectedOutputs: []string{}},
		{ID: "pay-1", Title: "Payment task 1", Priority: 1, Labels: []string{"spec:payments"}, ExpectedOutputs: []string{}},
		{ID: "other-1", Title: "Other task", Priority: 0, Labels: []string{"spec:other"}, ExpectedOutputs: []string{}},
	}

	// returnedBeads tracks which beads have been selected by the mock.
	// Since noopStage never closes beads, we use this instead of closedBeads
	// to prevent the same bead from being returned on every iteration.
	returnedBeads := make(map[string]bool)
	var selectedBeads []string
	var readyWithLabelCalls []string
	readyCalled := false

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			readyCalled = true
			t.Error("Ready() should never be called when label filters are set")
			return nil, nil
		},
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			readyWithLabelCalls = append(readyWithLabelCalls, label)
			b := selectNextBeadWithLabel(allBeads, returnedBeads, label)
			if b != nil {
				returnedBeads[b.ID] = true
				selectedBeads = append(selectedBeads, b.ID)
			}
			return b, nil
		},
	}

	cfg := &config.Config{}
	h := NewOrchestratorTestHelperWithDeps(t, cfg, io.Discard, mockBeads, newMockRouter())
	h.SetLabelFilters([]string{"spec:auth", "spec:payments"})

	ctx := context.Background()
	err := h.Run(ctx, 10, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if readyCalled {
		t.Error("Ready() was called during execution, but should never be called when label filters are active")
	}

	if len(selectedBeads) != 3 {
		t.Errorf("Expected 3 beads to be selected, got %d: %v", len(selectedBeads), selectedBeads)
	}

	if slices.Contains(selectedBeads, "other-1") {
		t.Error("Bead 'other-1' should not have been selected as it doesn't match label filters")
	}

	if len(readyWithLabelCalls) == 0 {
		t.Error("Expected ReadyWithLabel to be called, but it wasn't")
	}
}

// selectNextBeadWithLabel returns the highest-priority unclosed bead matching the given label.
func selectNextBeadWithLabel(allBeads []*bead.Bead, closedBeads map[string]bool, label string) *bead.Bead {
	var bestBead *bead.Bead
	for _, b := range allBeads {
		if closedBeads[b.ID] {
			continue
		}
		if !slices.Contains(b.Labels, label) {
			continue
		}
		if bestBead == nil || b.Priority < bestBead.Priority {
			bestBead = b
		}
	}
	return bestBead
}
