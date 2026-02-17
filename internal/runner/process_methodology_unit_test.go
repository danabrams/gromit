package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// newMinimalRunnerForMethodology creates the smallest possible Runner for
// testing prepareMethodologyForBead without needing a full Deps setup.
// Returns the runner and a pointer to the log buffer for output inspection.
func newMinimalRunnerForMethodology(t *testing.T, cfg *config.Config, renderer PromptRenderer) (*Runner, *strings.Builder) {
	t.Helper()
	if cfg == nil {
		cfg = &config.Config{}
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	buf := &strings.Builder{}
	sw := newSyncWriter(buf)
	r := &Runner{
		cfg:      cfg,
		renderer: renderer,
		output:   sw,
		syncOut:  sw,
	}
	return r, buf
}

// newBeadContext creates a minimal BeadContext for testing prepareMethodologyForBead.
func newBeadContextForMethodology(b *bead.Bead) *runtypes.BeadContext {
	return &runtypes.BeadContext{
		Bead:      b,
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{},
	}
}

// --- ATDD skip tests ---

// TestPrepareMethodology_ATDDSkippedForTestOnlyBead verifies that
// prepareMethodologyForBead returns atddActive=false when the bead title
// matches a test-only pattern, even when ATDD is globally enabled.
func TestPrepareMethodology_ATDDSkippedForTestOnlyBead(t *testing.T) {
	titles := []string{
		"Add tests for bead validation",
		"Add unit tests for config loading",
		"Write tests for runner loop",
		"Write unit tests for prompt rendering",
	}

	for _, title := range titles {
		t.Run(title, func(t *testing.T) {
			cfg := &config.Config{
				Methodology: config.MethodologyConfig{ATDD: true},
			}
			r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
			b := &bead.Bead{
				ID:     "test-skip-1",
				Title:  title,
				Labels: []string{},
			}
			bc := newBeadContextForMethodology(b)

			atddActive, _, _ := r.prepareMethodologyForBead(context.Background(), bc)

			if atddActive {
				t.Errorf("prepareMethodologyForBead should skip ATDD for test-only bead title %q, got atddActive=true", title)
			}
		})
	}
}

// TestPrepareMethodology_ATDDSkipLogsReason verifies that when ATDD is skipped
// for a test-only bead, prepareMethodologyForBead logs the reason.
func TestPrepareMethodology_ATDDSkipLogsReason(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{ATDD: true},
	}
	r, buf := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := &bead.Bead{
		ID:     "test-log-1",
		Title:  "Add tests for prompt rendering",
		Labels: []string{},
	}
	bc := newBeadContextForMethodology(b)

	r.prepareMethodologyForBead(context.Background(), bc)

	if !strings.Contains(buf.String(), "Skipping ATDD: bead is test-only") {
		t.Errorf("expected log 'Skipping ATDD: bead is test-only' in output, got:\n%s", buf.String())
	}
}

// --- TDD prompt routing tests ---

// TestPrepareMethodology_TDDSelectsRenderTDDBuild verifies that when TDD is
// globally enabled, prepareMethodologyForBead sets bc.BuildPrompt from
// RenderTDDBuild rather than leaving it empty (which means RenderBuild is used).
func TestPrepareMethodology_TDDSelectsRenderTDDBuild(t *testing.T) {
	tddBuildCalled := false
	renderer := &mockPromptRenderer{
		RenderTDDBuildFn: func(ctx *prompt.Context) (string, error) {
			tddBuildCalled = true
			return "tdd-build-prompt", nil
		},
	}
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{TDD: true},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, renderer)
	b := &bead.Bead{
		ID:     "tdd-routing-1",
		Title:  "Implement feature X",
		Labels: []string{},
	}
	bc := newBeadContextForMethodology(b)

	_, tddActive, _ := r.prepareMethodologyForBead(context.Background(), bc)

	if !tddActive {
		t.Error("prepareMethodologyForBead should return tddActive=true when TDD is enabled")
	}
	if !tddBuildCalled {
		t.Error("prepareMethodologyForBead should call RenderTDDBuild when TDD is active")
	}
	if bc.BuildPrompt != "tdd-build-prompt" {
		t.Errorf("bc.BuildPrompt should be set from RenderTDDBuild, got %q", bc.BuildPrompt)
	}
}

// TestPrepareMethodology_TDDInactiveDoesNotSetBuildPrompt verifies that when
// neither TDD nor ATDD is active, prepareMethodologyForBead does not set
// bc.BuildPrompt, leaving the caller to use RenderBuild.
func TestPrepareMethodology_TDDInactiveDoesNotSetBuildPrompt(t *testing.T) {
	tddBuildCalled := false
	renderer := &mockPromptRenderer{
		RenderTDDBuildFn: func(ctx *prompt.Context) (string, error) {
			tddBuildCalled = true
			return "tdd-build-prompt", nil
		},
	}
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{TDD: false, ATDD: false},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, renderer)
	b := &bead.Bead{
		ID:     "no-tdd-1",
		Title:  "Implement feature Y",
		Labels: []string{},
	}
	bc := newBeadContextForMethodology(b)

	_, tddActive, _ := r.prepareMethodologyForBead(context.Background(), bc)

	if tddActive {
		t.Error("prepareMethodologyForBead should return tddActive=false when TDD is disabled")
	}
	if tddBuildCalled {
		t.Error("prepareMethodologyForBead should not call RenderTDDBuild when TDD is inactive")
	}
	if bc.BuildPrompt != "" {
		t.Errorf("bc.BuildPrompt should remain empty when TDD is inactive, got %q", bc.BuildPrompt)
	}
}

// TestPrepareMethodology_ATDDSkipDoesNotCallRenderAcceptanceTests verifies that
// when ATDD is skipped for a test-only bead, RenderAcceptanceTests is never
// invoked on the renderer. Uses mock tracking to confirm the skip is complete.
func TestPrepareMethodology_ATDDSkipDoesNotCallRenderAcceptanceTests(t *testing.T) {
	renderAcceptanceCalled := false
	renderer := &mockPromptRenderer{
		RenderAcceptanceTestsFn: func(ctx *prompt.Context) (string, error) {
			renderAcceptanceCalled = true
			return "acceptance-tests-prompt", nil
		},
	}
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{ATDD: true},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, renderer)
	b := &bead.Bead{
		ID:     "test-no-render-1",
		Title:  "Add tests for runner loop",
		Labels: []string{},
	}
	bc := newBeadContextForMethodology(b)

	r.prepareMethodologyForBead(context.Background(), bc)

	if renderAcceptanceCalled {
		t.Error("RenderAcceptanceTests should not be called when ATDD is skipped for a test-only bead")
	}
}
