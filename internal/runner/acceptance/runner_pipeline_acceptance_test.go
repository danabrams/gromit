//go:build acceptance

package acceptance_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner"
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
				RenderBuildFn: func(_ *prompt.Context) (string, error) {
					return "standard build prompt", nil
				},
				RenderTDDBuildFn: func(_ *prompt.Context) (string, error) {
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
			beadReady := false
			r, err := runner.NewRunnerWithDeps(cfg, &buf, t.TempDir(),
				runner.Deps{
					Beads: &mockBeadClient{
						ReadyFn: func() (*bead.Bead, error) {
							if beadReady {
								return nil, nil
							}
							beadReady = true
							return &bead.Bead{
								ID:              "test-bead-1",
								Title:           "Test bead",
								Priority:        1,
								Labels:          tt.beadLabels,
								ExpectedOutputs: []string{},
							}, nil
						},
					},
					Router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
					Analyzer: &mockFailureAnalyzer{},
					Renderer: mockRenderer,
					Logger:   &mockIterationLogger{},
				})
			if err != nil {
				t.Fatalf("Failed to create runner: %v", err)
			}

			// Run the loop with one bead - it will process the bead then stop
			_ = r.Run(context.Background(), 0, time.Time{}, nil, false)

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

// TestOrchestratorHelper_TDDPromptSelection verifies that the orchestrator loop
// runs a bead to completion for each TDD config scenario without error.
func TestOrchestratorHelper_TDDPromptSelection(t *testing.T) {
	tests := []struct {
		name                 string
		globalTDD            bool
		beadLabels           []string
		expectTDDBuildCalled bool
	}{
		{name: "TDD active via global config", globalTDD: true, beadLabels: []string{}, expectTDDBuildCalled: true},
		{name: "TDD active via bead label", globalTDD: false, beadLabels: []string{"tdd:true"}, expectTDDBuildCalled: true},
		{name: "TDD inactive globally, no label", globalTDD: false, beadLabels: []string{}, expectTDDBuildCalled: false},
		{name: "TDD disabled via label overriding global", globalTDD: true, beadLabels: []string{"tdd:false"}, expectTDDBuildCalled: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tddBuildCalled := false
			cfg := &config.Config{
				Methodology: config.MethodologyConfig{TDD: tt.globalTDD},
			}
			beadReady := false
			mockBeads := &mockBeadClient{
				ReadyFn: func() (*bead.Bead, error) {
					if beadReady {
						return nil, nil
					}
					beadReady = true
					return &bead.Bead{
						ID:     "test-bead-1",
						Title:  "Test bead",
						Labels: tt.beadLabels,
					}, nil
				},
			}
			h := NewOrchestratorTestHelperWithDeps(t, cfg, io.Discard, mockBeads, newMockRouter())
			if err := h.Run(context.Background(), 0, time.Time{}, nil); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if tt.expectTDDBuildCalled && !tddBuildCalled {
				t.Errorf("%s: Expected RenderTDDBuild to be called but it wasn't", tt.name)
			}
			if !tt.expectTDDBuildCalled && tddBuildCalled {
				t.Errorf("%s: Expected RenderTDDBuild NOT to be called but it was", tt.name)
			}
		})
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
				RenderBuildFn: func(_ *prompt.Context) (string, error) {
					return "standard build prompt", nil
				},
				RenderAcceptanceTestsFn: func(_ *prompt.Context) (string, error) {
					acceptanceTestsCalled = true
					return "acceptance tests prompt", nil
				},
				RenderATDDBuildFn: func(_ *prompt.Context) (string, error) {
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
			beadReady := false
			r, err := runner.NewRunnerWithDeps(cfg, &buf, t.TempDir(),
				runner.Deps{
					Beads: &mockBeadClient{
						ReadyFn: func() (*bead.Bead, error) {
							if beadReady {
								return nil, nil
							}
							beadReady = true
							return &bead.Bead{
								ID:              "test-bead-1",
								Title:           tt.beadTitle,
								Priority:        1,
								Labels:          tt.beadLabels,
								ExpectedOutputs: []string{},
							}, nil
						},
					},
					Router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
					Analyzer: &mockFailureAnalyzer{},
					Renderer: mockRend,
					Logger:   &mockIterationLogger{},
				})
			if err != nil {
				t.Fatalf("Failed to create runner: %v", err)
			}

			_ = r.Run(context.Background(), 0, time.Time{}, nil, false)

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
