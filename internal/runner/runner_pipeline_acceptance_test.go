//go:build acceptance

package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
)

func TestTDDPromptSelection(t *testing.T) {
	buildContextFn := testPromptContext
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
				BuildContextFn: buildContextFn,
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

// TestATDDSkippedForTestOnlyBead verifies that when a bead's title matches the
// test-only heuristic, the ATDD pre-pass is automatically skipped even when ATDD
// is globally active. This covers acceptance criteria 2 and 3:
// - AC2: Beads whose title matches test-only heuristic skip ATDD
// - AC3: When ATDD is skipped for a test-only bead, log the reason
func TestATDDSkippedForTestOnlyBead(t *testing.T) {
	buildContextFn := testPromptContext
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
				BuildContextFn: buildContextFn,
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

func testPromptContext(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
	return &prompt.Context{
		Bead:               b,
		ParentBead:         parent,
		Iteration:          iteration,
		Model:              model,
		ConfirmedLearnings: []learnings.Learning{},
		RecentLearnings:    []learnings.Learning{},
	}, nil
}
