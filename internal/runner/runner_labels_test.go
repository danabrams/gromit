package runner

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
)

// TestRunner_AcceptsLabelFilters tests that Runner can accept optional spec labels
func TestRunner_AcceptsLabelFilters(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	mock := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return nil, nil // No work
		},
	}

	deps := Deps{
		Beads:    mock,
		Router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	}

	r, err := NewRunnerWithDeps(cfg, io.Discard, "/tmp/gromit", deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() error = %v", err)
	}

	// Test that Runner can be configured with label filters
	labels := []string{"spec:auth", "spec:payments"}
	r.SetLabelFilters(labels)

	// Verify labels were stored
	if len(r.labelFilters) != 2 {
		t.Errorf("Expected 2 label filters, got %d", len(r.labelFilters))
	}
	if r.labelFilters[0] != "spec:auth" {
		t.Errorf("Expected first label 'spec:auth', got %s", r.labelFilters[0])
	}
	if r.labelFilters[1] != "spec:payments" {
		t.Errorf("Expected second label 'spec:payments', got %s", r.labelFilters[1])
	}
}

// TestRunner_NoFiltersUsesReady tests that when no filters are set, Runner uses Ready()
func TestRunner_NoFiltersUsesReady(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	readyCalled := false
	readyWithLabelCalled := false

	callCount := 0
	mock := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			readyCalled = true
			callCount++
			// Return a bead on first call, nil on second
			if callCount == 1 {
				return &bead.Bead{
					ID:              "task-1",
					Title:           "Regular task",
					Priority:        1,
					Labels:          []string{},
					ExpectedOutputs: []string{},
				}, nil
			}
			return nil, nil
		},
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			readyWithLabelCalled = true
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

	// Do NOT set label filters (leave it empty/nil)

	// Run the loop
	err = r.Run(context.Background(), 0, time.Time{}, nil, false)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify Ready was called, not ReadyWithLabel
	if !readyCalled {
		t.Error("Expected Ready() to be called")
	}
	if readyWithLabelCalled {
		t.Error("Expected ReadyWithLabel() NOT to be called when no filters set")
	}

	// Verify the bead was processed and closed
	if len(mock.ClosedIDs) != 1 || mock.ClosedIDs[0] != "task-1" {
		t.Errorf("Expected bead 'task-1' to be closed, got: %v", mock.ClosedIDs)
	}
}
