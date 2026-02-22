//go:build acceptance

package acceptance_test

import (
	"context"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner"
)

func TestScopedRun_FullLoopWithLabelFilters(t *testing.T) {
	// Create beads that will be processed across multiple iterations
	allBeads := []*bead.Bead{
		{ID: "auth-1", Title: "Auth task 1", Priority: 1, Labels: []string{"spec:auth"}, ExpectedOutputs: []string{}},
		{ID: "auth-2", Title: "Auth task 2", Priority: 0, Labels: []string{"spec:auth"}, ExpectedOutputs: []string{}},
		{ID: "pay-1", Title: "Payment task 1", Priority: 1, Labels: []string{"spec:payments"}, ExpectedOutputs: []string{}},
		{ID: "other-1", Title: "Other task", Priority: 0, Labels: []string{"spec:other"}, ExpectedOutputs: []string{}},
	}

	var processedBeads []string
	closedBeads := make(map[string]bool)
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
			return selectNextBeadWithLabel(allBeads, closedBeads, label), nil
		},
		CloseFn: func(id string) error {
			closedBeads[id] = true
			processedBeads = append(processedBeads, id)
			return nil
		},
		SyncFn: func() error {
			return nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "done"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "validation passed"}, nil
		},
	}

	precheckDisabled := false
	autoPushDisabled := false

	cfg := &config.Config{
		Loop: config.LoopConfig{
			StopOnFailure: false,
		},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
		Precheck: config.PrecheckConfig{
			Enabled: &precheckDisabled,
		},
		Review: config.ReviewConfig{
			Enabled: false,
		},
		Git: config.GitConfig{
			AutoPush: &autoPushDisabled,
		},
	}

	var buf strings.Builder
	r, err := runner.NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		runner.Deps{
			Beads:    mockBeads,
			Router:   newMockRouterFromClaudeClient(mockClaude),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
		})
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	// Set label filters for auth and payments only (not "other")
	r.SetLabelFilters([]string{"spec:auth", "spec:payments"})

	ctx := context.Background()
	err = r.Run(ctx, 10, time.Time{}, nil, false)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify Ready() was never called across all iterations
	if readyCalled {
		t.Error("Ready() was called during execution, but should never be called when label filters are active")
	}

	// Verify only auth and payment beads were processed (not "other-1")
	expectedBeads := []string{"auth-2", "auth-1", "pay-1"} // Priority order: P0, P1, P1
	if len(processedBeads) != len(expectedBeads) {
		t.Errorf("Expected %d beads to be processed, got %d: %v", len(expectedBeads), len(processedBeads), processedBeads)
	}

	// Verify other-1 was NOT processed (it has spec:other label which is not in filters)
	if slices.Contains(processedBeads, "other-1") {
		t.Error("Bead 'other-1' should not have been processed as it doesn't match label filters")
	}

	// Verify ReadyWithLabel was called for each label
	if len(readyWithLabelCalls) == 0 {
		t.Error("Expected ReadyWithLabel to be called, but it wasn't")
	}
}

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
