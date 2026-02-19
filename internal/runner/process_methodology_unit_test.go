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

const atddSkipLogMessage = "Skipping ATDD: bead is test-only"
const atddSpecGranularitySkipLogMessage = "Skipping ATDD: spec granularity active for spec:"

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

func newTestBead(id, title string) *bead.Bead {
	return &bead.Bead{
		ID:     id,
		Title:  title,
		Labels: []string{},
	}
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
			b := newTestBead("test-skip-1", title)
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
	b := newTestBead("test-log-1", "Add tests for prompt rendering")
	bc := newBeadContextForMethodology(b)

	r.prepareMethodologyForBead(context.Background(), bc)

	if !strings.Contains(buf.String(), atddSkipLogMessage) {
		t.Errorf("expected log %q in output, got:\n%s", atddSkipLogMessage, buf.String())
	}
}

// TestPrepareMethodology_ATDDSkippedForSpecGranularity verifies that when
// granularity is set to spec and a bead has a spec label, ATDD is skipped.
func TestPrepareMethodology_ATDDSkippedForSpecGranularity(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			ATDD:        true,
			Granularity: config.MethodologyGranularitySpec,
		},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := newTestBead("spec-skip-1", "Implement feature")
	b.Labels = []string{"spec:auth"}
	bc := newBeadContextForMethodology(b)

	atddActive, _, _ := r.prepareMethodologyForBead(context.Background(), bc)

	if atddActive {
		t.Error("prepareMethodologyForBead should skip ATDD when granularity=spec and bead has spec label")
	}
}

// TestPrepareMethodology_ATDDSkipSpecGranularityLogsReason verifies that when
// ATDD is skipped for spec granularity, the reason is logged.
func TestPrepareMethodology_ATDDSkipSpecGranularityLogsReason(t *testing.T) {
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			ATDD:        true,
			Granularity: config.MethodologyGranularitySpec,
		},
	}
	r, buf := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := newTestBead("spec-log-1", "Implement feature")
	b.Labels = []string{"spec:auth"}
	bc := newBeadContextForMethodology(b)

	r.prepareMethodologyForBead(context.Background(), bc)

	if !strings.Contains(buf.String(), atddSpecGranularitySkipLogMessage) {
		t.Errorf("expected log %q in output, got:\n%s", atddSpecGranularitySkipLogMessage, buf.String())
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
	b := newTestBead("tdd-routing-1", "Implement feature X")
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

type mockTDDOrchestrator struct {
	runCyclesFn func(ctx context.Context, bc *runtypes.BeadContext) error
}

func (m *mockTDDOrchestrator) RunCycles(ctx context.Context, bc *runtypes.BeadContext) error {
	if m.runCyclesFn != nil {
		return m.runCyclesFn(ctx, bc)
	}
	return nil
}

func TestPrepareMethodology_TDDFreshContextRunsOrchestrator(t *testing.T) {
	tddBuildCalled := false
	orchestratorCalled := false
	renderer := &mockPromptRenderer{
		RenderTDDBuildFn: func(ctx *prompt.Context) (string, error) {
			tddBuildCalled = true
			return "tdd-build-prompt", nil
		},
	}
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			TDD:                  true,
			FreshContextPerCycle: true,
		},
	}
	r, _ := newMinimalRunnerForMethodology(t, cfg, renderer)
	r.tddOrchestrator = &mockTDDOrchestrator{
		runCyclesFn: func(ctx context.Context, bc *runtypes.BeadContext) error {
			orchestratorCalled = true
			return nil
		},
	}
	b := newTestBead("tdd-fresh-context-1", "Implement feature with cycle orchestration")
	bc := newBeadContextForMethodology(b)

	_, tddActive, done := r.prepareMethodologyForBead(context.Background(), bc)

	if !tddActive {
		t.Fatal("prepareMethodologyForBead should return tddActive=true when TDD is enabled")
	}
	if !done {
		t.Fatal("prepareMethodologyForBead should return done=true after tdd orchestrator success")
	}
	if !orchestratorCalled {
		t.Fatal("prepareMethodologyForBead should call tddOrchestrator.RunCycles when fresh_context_per_cycle=true")
	}
	if tddBuildCalled {
		t.Fatal("prepareMethodologyForBead should not call RenderTDDBuild when fresh_context_per_cycle=true")
	}
	if bc.Result.Error != nil {
		t.Fatalf("bc.Result.Error = %v, want nil", bc.Result.Error)
	}
	if !bc.Result.Success {
		t.Fatal("bc.Result.Success should be true when tddOrchestrator.RunCycles succeeds")
	}
	if !bc.Result.FirstPassSuccess {
		t.Fatal("bc.Result.FirstPassSuccess should be true when tddOrchestrator.RunCycles succeeds")
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
	b := newTestBead("no-tdd-1", "Implement feature Y")
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
	b := newTestBead("test-no-render-1", "Add tests for runner loop")
	bc := newBeadContextForMethodology(b)

	r.prepareMethodologyForBead(context.Background(), bc)

	if renderAcceptanceCalled {
		t.Error("RenderAcceptanceTests should not be called when ATDD is skipped for a test-only bead")
	}
}

// --- FirstPassSuccess tests ---

// TestExecuteBuildLoop_InjectsScopedTestCommandFromTouchedPackages verifies that
// executeBuildAndMethodologyLoop calls injectScopedTestCommand before invoking
// executeWithRetry, so the prompt context reflects the current touched packages.
func TestExecuteBuildLoop_InjectsScopedTestCommandFromTouchedPackages(t *testing.T) {
	cfg := &config.Config{}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := newTestBead("inject-1", "Implement feature")
	bc := newBeadContextForMethodology(b)
	bc.TouchedPackages = []string{"internal/runner", "internal/config"}

	var capturedScopedCmd string
	result := r.executeBuildAndMethodologyLoop(
		context.Background(), bc,
		false, false,
		func() bool {
			if bc.PromptCtx != nil {
				capturedScopedCmd = bc.PromptCtx.ScopedTestCommand
			}
			return true
		},
	)

	if !result.Success {
		t.Fatal("expected Success=true")
	}
	want := "go test ./internal/runner/... ./internal/config/..."
	if capturedScopedCmd != want {
		t.Errorf("ScopedTestCommand before executeWithRetry = %q, want %q", capturedScopedCmd, want)
	}
}

// TestExecuteBuildLoop_ScopedTestCommandEmptyWhenNoPackages verifies that
// executeBuildAndMethodologyLoop does not inject a ScopedTestCommand when
// TouchedPackages is empty.
func TestExecuteBuildLoop_ScopedTestCommandEmptyWhenNoPackages(t *testing.T) {
	cfg := &config.Config{}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := newTestBead("inject-2", "Implement feature")
	bc := newBeadContextForMethodology(b)
	// No TouchedPackages set

	var capturedScopedCmd string
	result := r.executeBuildAndMethodologyLoop(
		context.Background(), bc,
		false, false,
		func() bool {
			if bc.PromptCtx != nil {
				capturedScopedCmd = bc.PromptCtx.ScopedTestCommand
			}
			return true
		},
	)

	if !result.Success {
		t.Fatal("expected Success=true")
	}
	if capturedScopedCmd != "" {
		t.Errorf("ScopedTestCommand should be empty when no packages touched, got %q", capturedScopedCmd)
	}
}

// TestExecuteBuildLoop_FirstPassSuccess_NoRetriesNoEscalation verifies that
// FirstPassSuccess is set to true when the build succeeds on the first attempt
// without any retries or escalation.
func TestExecuteBuildLoop_FirstPassSuccess_NoRetriesNoEscalation(t *testing.T) {
	cfg := &config.Config{}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := newTestBead("fps-1", "Implement feature")
	bc := newBeadContextForMethodology(b)

	result := r.executeBuildAndMethodologyLoop(
		context.Background(), bc,
		false, false, // no ATDD, no TDD
		func() bool { return true }, // build succeeds immediately
	)

	if !result.Success {
		t.Fatal("expected Success=true")
	}
	if !result.FirstPassSuccess {
		t.Error("expected FirstPassSuccess=true when no retries and no escalation")
	}
}

// TestExecuteBuildLoop_FirstPassSuccess_FalseAfterRetry verifies that
// FirstPassSuccess remains false when the build succeeded but required retries.
func TestExecuteBuildLoop_FirstPassSuccess_FalseAfterRetry(t *testing.T) {
	cfg := &config.Config{}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := newTestBead("fps-2", "Implement feature with retry")
	bc := newBeadContextForMethodology(b)
	bc.TotalRetriesThisBead = 1 // simulate a retry happened

	result := r.executeBuildAndMethodologyLoop(
		context.Background(), bc,
		false, false,
		func() bool { return true },
	)

	if !result.Success {
		t.Fatal("expected Success=true")
	}
	if result.FirstPassSuccess {
		t.Error("expected FirstPassSuccess=false when retries occurred")
	}
}

// TestExecuteBuildLoop_FirstPassSuccess_FalseAfterEscalation verifies that
// FirstPassSuccess remains false when the build succeeded but required escalation.
func TestExecuteBuildLoop_FirstPassSuccess_FalseAfterEscalation(t *testing.T) {
	cfg := &config.Config{}
	r, _ := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
	b := newTestBead("fps-3", "Implement feature with escalation")
	bc := newBeadContextForMethodology(b)
	bc.Result.Escalated = true // simulate escalation happened

	result := r.executeBuildAndMethodologyLoop(
		context.Background(), bc,
		false, false,
		func() bool { return true },
	)

	if !result.Success {
		t.Fatal("expected Success=true")
	}
	if result.FirstPassSuccess {
		t.Error("expected FirstPassSuccess=false when escalation occurred")
	}
}
