//go:build acceptance

package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
)

func TestTDDPromptSelection(t *testing.T) {
	tests := []struct {
		name                 string
		globalTDD            bool
		beadLabels           []string
		expectTDDBuildCalled bool
		description          string
	}{
		{
			name:                 "TDD active via global config - RenderTDDBuild called",
			globalTDD:            true,
			beadLabels:           []string{},
			expectTDDBuildCalled: true,
			description:          "When global TDD is true and bead has no tdd label, RenderTDDBuild should be called",
		},
		{
			name:                 "TDD active via bead label - RenderTDDBuild called",
			globalTDD:            false,
			beadLabels:           []string{"tdd:true"},
			expectTDDBuildCalled: true,
			description:          "When bead has tdd:true label, RenderTDDBuild should be called regardless of global config",
		},
		{
			name:                 "TDD inactive globally, no label - RenderTDDBuild not called",
			globalTDD:            false,
			beadLabels:           []string{},
			expectTDDBuildCalled: false,
			description:          "When TDD is not active, RenderTDDBuild should not be called",
		},
		{
			name:                 "TDD disabled via label overriding global - RenderTDDBuild not called",
			globalTDD:            true,
			beadLabels:           []string{"tdd:false"},
			expectTDDBuildCalled: false,
			description:          "When bead has tdd:false label, RenderTDDBuild should not be called even if global TDD is true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tddBuildCalled := false

			mockRenderer := &mockPromptRenderer{
				BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
					return &prompt.Context{
						Bead:               b,
						ParentBead:         parent,
						Iteration:          iteration,
						Model:              model,
						ConfirmedLearnings: []learnings.Learning{},
						RecentLearnings:    []learnings.Learning{},
					}, nil
				},
				RenderBuildFn: func(ctx *prompt.Context) (string, error) {
					return "standard build prompt", nil
				},
				RenderTDDBuildFn: func(ctx *prompt.Context) (string, error) {
					tddBuildCalled = true
					return "tdd build prompt", nil
				},
			}

			cfg := &config.Config{
				Methodology: config.MethodologyConfig{
					TDD: tt.globalTDD,
				},
				Validation: config.ValidationConfig{
					Enabled: false, // Disable validation to isolate prompt selection
				},
			}

			var buf strings.Builder
			r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
				Deps{
					Beads:    &mockBeadClient{},
					Router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
					Analyzer: &mockFailureAnalyzer{},
					Renderer: mockRenderer,
					Logger:   &mockIterationLogger{},
				})
			if err != nil {
				t.Fatalf("Failed to create runner: %v", err)
			}

			testBead := &bead.Bead{
				ID:       "test-bead-1",
				Title:    "Test bead",
				Priority: 1,
				Labels:   tt.beadLabels,
			}

			// Call processBead - we don't care about the result, just whether RenderTDDBuild was called
			_ = r.processBead(context.Background(), testBead, 1, time.Time{}, nil)

			// Verify expectations
			if tt.expectTDDBuildCalled && !tddBuildCalled {
				t.Errorf("%s: Expected RenderTDDBuild to be called but it wasn't", tt.description)
			}
			if !tt.expectTDDBuildCalled && tddBuildCalled {
				t.Errorf("%s: Expected RenderTDDBuild NOT to be called but it was", tt.description)
			}
		})
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

			// Find the highest priority non-closed bead with this label
			var bestBead *bead.Bead
			for i := 0; i < len(allBeads); i++ {
				b := allBeads[i]
				if closedBeads[b.ID] {
					continue
				}
				// Check if this bead has the requested label
				hasLabel := false
				for _, l := range b.Labels {
					if l == label {
						hasLabel = true
						break
					}
				}
				if !hasLabel {
					continue
				}
				// Select highest priority (lowest number)
				if bestBead == nil || b.Priority < bestBead.Priority {
					bestBead = b
				}
			}
			return bestBead, nil
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

	// Verify beads were processed in priority order
	if len(processedBeads) == len(expectedBeads) {
		for i, expected := range expectedBeads {
			if processedBeads[i] != expected {
				t.Errorf("Expected bead at position %d to be %q, got %q", i, expected, processedBeads[i])
			}
		}
	}

	// Verify ReadyWithLabel was called multiple times across iterations
	if len(readyWithLabelCalls) < 3 {
		t.Errorf("Expected ReadyWithLabel to be called at least 3 times (across multiple iterations), got %d calls", len(readyWithLabelCalls))
	}

	// Verify both labels were queried in the calls
	foundAuth := false
	foundPayments := false
	for _, label := range readyWithLabelCalls {
		if label == "spec:auth" {
			foundAuth = true
		}
		if label == "spec:payments" {
			foundPayments = true
		}
	}
	if !foundAuth {
		t.Error("Expected ReadyWithLabel to be called with 'spec:auth' label")
	}
	if !foundPayments {
		t.Error("Expected ReadyWithLabel to be called with 'spec:payments' label")
	}
}

// TestATDDSkippedForTestOnlyBead verifies that when a bead's title matches the
// test-only heuristic, the ATDD pre-pass is automatically skipped even when ATDD
// is globally active. This covers acceptance criteria 2 and 3:
// - AC2: Beads whose title matches test-only heuristic skip ATDD
// - AC3: When ATDD is skipped for a test-only bead, log the reason
func TestATDDSkippedForTestOnlyBead(t *testing.T) {
	tests := []struct {
		name                        string
		beadTitle                   string
		globalATDD                  bool
		beadLabels                  []string
		expectAcceptanceTestsCalled bool
		expectSkipLogMessage        bool
		description                 string
	}{
		{
			name:                        "test-only bead with global ATDD skips ATDD",
			beadTitle:                   "Add unit tests for config loading",
			globalATDD:                  true,
			beadLabels:                  []string{},
			expectAcceptanceTestsCalled: false,
			expectSkipLogMessage:        true,
			description:                 "A bead titled 'Add unit tests for X' should skip ATDD even when globally enabled",
		},
		{
			name:                        "test-only bead with Add tests prefix skips ATDD",
			beadTitle:                   "Add tests for runner escalation",
			globalATDD:                  true,
			beadLabels:                  []string{},
			expectAcceptanceTestsCalled: false,
			expectSkipLogMessage:        true,
			description:                 "A bead titled 'Add tests for X' should skip ATDD",
		},
		{
			name:                        "test-only bead with Write tests prefix skips ATDD",
			beadTitle:                   "Write tests for prompt rendering",
			globalATDD:                  true,
			beadLabels:                  []string{},
			expectAcceptanceTestsCalled: false,
			expectSkipLogMessage:        true,
			description:                 "A bead titled 'Write tests for X' should skip ATDD",
		},
		{
			name:                        "regular bead with global ATDD runs ATDD",
			beadTitle:                   "Implement dark mode toggle",
			globalATDD:                  true,
			beadLabels:                  []string{},
			expectAcceptanceTestsCalled: true,
			expectSkipLogMessage:        false,
			description:                 "A non-test bead should still run ATDD when globally enabled",
		},
		{
			name:                        "test-only bead with ATDD disabled globally does not log skip",
			beadTitle:                   "Add unit tests for config loading",
			globalATDD:                  false,
			beadLabels:                  []string{},
			expectAcceptanceTestsCalled: false,
			expectSkipLogMessage:        false,
			description:                 "When ATDD is globally disabled, test-only beads don't need the skip log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acceptanceTestsCalled := false

			mockRend := &mockPromptRenderer{
				BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
					return &prompt.Context{
						Bead:               b,
						ParentBead:         parent,
						Iteration:          iteration,
						Model:              model,
						ConfirmedLearnings: []learnings.Learning{},
						RecentLearnings:    []learnings.Learning{},
					}, nil
				},
				RenderBuildFn: func(ctx *prompt.Context) (string, error) {
					return "standard build prompt", nil
				},
				RenderAcceptanceTestsFn: func(ctx *prompt.Context) (string, error) {
					acceptanceTestsCalled = true
					return "acceptance tests prompt", nil
				},
				RenderATDDBuildFn: func(ctx *prompt.Context) (string, error) {
					return "atdd build prompt", nil
				},
			}

			cfg := &config.Config{
				Methodology: config.MethodologyConfig{
					ATDD: tt.globalATDD,
				},
				Validation: config.ValidationConfig{
					Enabled: false,
				},
			}

			var buf strings.Builder
			r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
				Deps{
					Beads:    &mockBeadClient{},
					Router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
					Analyzer: &mockFailureAnalyzer{},
					Renderer: mockRend,
					Logger:   &mockIterationLogger{},
				})
			if err != nil {
				t.Fatalf("Failed to create runner: %v", err)
			}

			testBead := &bead.Bead{
				ID:       "test-bead-1",
				Title:    tt.beadTitle,
				Priority: 1,
				Labels:   tt.beadLabels,
			}

			_ = r.processBead(context.Background(), testBead, 1, time.Time{}, nil)

			output := buf.String()

			// AC2: Verify ATDD pre-pass was or was not called
			if tt.expectAcceptanceTestsCalled && !acceptanceTestsCalled {
				t.Errorf("%s: Expected RenderAcceptanceTests to be called but it wasn't", tt.description)
			}
			if !tt.expectAcceptanceTestsCalled && acceptanceTestsCalled {
				t.Errorf("%s: Expected RenderAcceptanceTests NOT to be called but it was", tt.description)
			}

			// AC3: Verify skip log message
			if tt.expectSkipLogMessage {
				if !strings.Contains(output, "Skipping ATDD: bead is test-only") {
					t.Errorf("%s: Expected log output to contain 'Skipping ATDD: bead is test-only', got: %s", tt.description, output)
				}
			} else {
				if strings.Contains(output, "Skipping ATDD: bead is test-only") {
					t.Errorf("%s: Expected log output NOT to contain 'Skipping ATDD: bead is test-only', got: %s", tt.description, output)
				}
			}
		})
	}
}
