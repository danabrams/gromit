//go:build acceptance

package runner

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
)

// TestRunner_UsesLabelFiltersInLoop tests that Runner calls ReadyWithLabel for each label
func TestRunner_UsesLabelFiltersInLoop(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Track which labels were queried
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
			// Return a bead for the first call only (first label, first iteration)
			if callCount == 1 && label == "spec:auth" {
				return &bead.Bead{
					ID:              "auth-1",
					Title:           "Auth task",
					Priority:        1,
					Labels:          []string{"spec:auth"},
					ExpectedOutputs: []string{},
				}, nil
			}
			// All other calls return nil (no more work)
			return nil, nil
		},
	}

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "done"}, nil
		},
	}

	deps := Deps{
		Beads:    mock,
		Router:   newMockRouterFromClaudeClient(mockClaude),
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	}

	r, err := NewRunnerWithDeps(cfg, io.Discard, "/tmp/gromit", deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() error = %v", err)
	}

	// Set label filters
	r.SetLabelFilters([]string{"spec:auth", "spec:payments"})

	// Run the loop
	err = r.Run(context.Background(), 0, time.Time{}, nil, false)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify ReadyWithLabel was called
	// The runner should iterate through labels on each loop iteration
	// Loop 1: tries auth (finds bead), processes it
	// Loop 2: tries auth (nil), tries payments (nil), exits
	if len(queriedLabels) < 2 {
		t.Fatalf("Expected at least 2 ReadyWithLabel calls, got %d: %v", len(queriedLabels), queriedLabels)
	}

	// Just verify both labels were queried at some point
	hasAuth := false
	hasPayments := false
	for _, label := range queriedLabels {
		if label == "spec:auth" {
			hasAuth = true
		}
		if label == "spec:payments" {
			hasPayments = true
		}
	}
	if !hasAuth {
		t.Error("Expected 'spec:auth' to be queried")
	}
	if !hasPayments {
		t.Error("Expected 'spec:payments' to be queried")
	}

	// Verify the auth bead was processed and closed
	if len(mock.ClosedIDs) != 1 || mock.ClosedIDs[0] != "auth-1" {
		t.Errorf("Expected bead 'auth-1' to be closed, got: %v", mock.ClosedIDs)
	}
}

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
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
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
	for _, id := range processedBeads {
		if id == "other-1" {
			t.Errorf("Bead 'other-1' should not have been processed as it doesn't match label filters")
		}
	}

	// Verify ReadyWithLabel was called for each label
	if len(readyWithLabelCalls) == 0 {
		t.Error("Expected ReadyWithLabel to be called, but it wasn't")
	}
}

func selectNextBeadWithLabel(allBeads []*bead.Bead, closedBeads map[string]bool, label string) *bead.Bead {
	var bestBead *bead.Bead
	for i := 0; i < len(allBeads); i++ {
		b := allBeads[i]
		if closedBeads[b.ID] {
			continue
		}
		if !hasLabel(b.Labels, label) {
			continue
		}
		if bestBead == nil || b.Priority < bestBead.Priority {
			bestBead = b
		}
	}
	return bestBead
}

func hasLabel(labels []string, label string) bool {
	for _, l := range labels {
		if l == label {
			return true
		}
	}
	return false
}
